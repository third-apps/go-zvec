package flat_sparse

import (
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type SparseVector struct {
	Indices []uint32
	Values  []float32
}

type SparseIndex struct {
	mu         sync.RWMutex
	vectors    []SparseVector
	pks        []string
	pkToDocID  map[string]int
	docIDs     []uint64
	deleted    []bool
	liveCount  int
	metricType types.MetricType
}

func NewSparseIndex(metricType types.MetricType) *SparseIndex {
	return &SparseIndex{
		vectors:    make([]SparseVector, 0),
		pks:        make([]string, 0),
		pkToDocID:  make(map[string]int),
		docIDs:     make([]uint64, 0),
		deleted:    make([]bool, 0),
		metricType: metricType,
	}
}

func (idx *SparseIndex) Add(indices []uint32, values []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := uint64(len(idx.vectors))
	sv := SparseVector{Indices: make([]uint32, len(indices)), Values: make([]float32, len(values))}
	copy(sv.Indices, indices)
	copy(sv.Values, values)
	idx.vectors = append(idx.vectors, sv)
	idx.pks = append(idx.pks, pk)
	idx.pkToDocID[pk] = len(idx.pks) - 1
	idx.docIDs = append(idx.docIDs, docID)
	idx.deleted = append(idx.deleted, false)
	idx.liveCount++
	return docID
}

func (idx *SparseIndex) Search(queryIndices []uint32, queryValues []float32, topK int) []types.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.liveCount == 0 {
		return nil
	}

	results := make([]types.SearchResult, 0, idx.liveCount)
	for i, sv := range idx.vectors {
		if idx.deleted[i] {
			continue
		}
		dist := metric.SparseInnerProduct(queryIndices, queryValues, sv.Indices, sv.Values)
		results = append(results, types.SearchResult{
			DocID: idx.docIDs[i],
			Score: 1.0 / (1.0 + dist),
			PK:    idx.pks[i],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

func (idx *SparseIndex) SearchWithFilter(queryIndices []uint32, queryValues []float32, topK int, filterFn func(pk string) bool) []types.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.liveCount == 0 {
		return nil
	}

	results := make([]types.SearchResult, 0, idx.liveCount)
	for i, sv := range idx.vectors {
		if idx.deleted[i] || !filterFn(idx.pks[i]) {
			continue
		}
		dist := metric.SparseInnerProduct(queryIndices, queryValues, sv.Indices, sv.Values)
		results = append(results, types.SearchResult{
			DocID: idx.docIDs[i],
			Score: 1.0 / (1.0 + dist),
			PK:    idx.pks[i],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > len(results) {
		topK = len(results)
	}
	return results[:topK]
}

func (idx *SparseIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	i, ok := idx.pkToDocID[pk]
	if !ok || idx.deleted[i] {
		return false
	}
	idx.deleted[i] = true
	idx.liveCount--
	delete(idx.pkToDocID, pk)
	return true
}

func (idx *SparseIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.liveCount
}

func (idx *SparseIndex) Close() error {
	return nil
}
