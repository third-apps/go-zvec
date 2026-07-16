package flat_sparse

import (
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
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
	docIDs     []uint64
	metricType types.MetricType
}

func NewSparseIndex(metricType types.MetricType) *SparseIndex {
	return &SparseIndex{
		vectors:    make([]SparseVector, 0),
		pks:        make([]string, 0),
		docIDs:     make([]uint64, 0),
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
	idx.docIDs = append(idx.docIDs, docID)
	return docID
}

func (idx *SparseIndex) Search(queryIndices []uint32, queryValues []float32, topK int) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.vectors) == 0 {
		return nil
	}

	results := make([]flat.SearchResult, 0, len(idx.vectors))
	for i, sv := range idx.vectors {
		dist := metric.SparseInnerProduct(queryIndices, queryValues, sv.Indices, sv.Values)
		results = append(results, flat.SearchResult{
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

func (idx *SparseIndex) SearchWithFilter(queryIndices []uint32, queryValues []float32, topK int, filterFn func(pk string) bool) []flat.SearchResult {
	all := idx.Search(queryIndices, queryValues, len(idx.vectors))
	var results []flat.SearchResult
	for _, r := range all {
		if filterFn(r.PK) {
			results = append(results, r)
			if len(results) >= topK {
				break
			}
		}
	}
	return results
}

func (idx *SparseIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.vectors = append(idx.vectors[:i], idx.vectors[i+1:]...)
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
			idx.docIDs = append(idx.docIDs[:i], idx.docIDs[i+1:]...)
			return true
		}
	}
	return false
}

func (idx *SparseIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.vectors)
}