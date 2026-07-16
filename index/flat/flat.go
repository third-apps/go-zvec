package flat

import (
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type SearchResult struct {
	DocID uint64
	Score float32
	PK    string
}

type FlatIndex struct {
	mu         sync.RWMutex
	vectors    [][]float32
	pks        []string
	dimension  int
	metricType types.MetricType
	distFn     metric.DistanceFunc
}

func NewFlatIndex(dimension int, metricType types.MetricType) *FlatIndex {
	return &FlatIndex{
		dimension:  dimension,
		metricType: metricType,
		distFn:     metric.GetDistanceFunc(metricType),
	}
}

func (idx *FlatIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := uint64(len(idx.vectors))
	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}
	idx.vectors = append(idx.vectors, v)
	idx.pks = append(idx.pks, pk)
	return docID
}

func (idx *FlatIndex) Search(query []float32, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.vectors) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	type distIdx struct {
		dist  float32
		index int
	}

	results := make([]distIdx, len(idx.vectors))
	for i, v := range idx.vectors {
		results[i] = distIdx{dist: idx.distFn(q, v), index: i}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].dist < results[j].dist
	})

	if topK > len(results) {
		topK = len(results)
	}

	final := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		r := results[i]
		final[i] = SearchResult{
			DocID: uint64(r.index),
			Score: 1.0 / (1.0 + r.dist),
			PK:    idx.pks[r.index],
		}
	}
	return final
}

func (idx *FlatIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []SearchResult {

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	type distIdx struct {
		dist  float32
		index int
	}

	var candidates []distIdx
	for i, v := range idx.vectors {
		if filterFn(idx.pks[i]) {
			candidates = append(candidates, distIdx{
				dist: idx.distFn(q, v), index: i,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	final := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		c := candidates[i]
		final[i] = SearchResult{
			DocID: uint64(c.index),
			Score: 1.0 / (1.0 + c.dist),
			PK:    idx.pks[c.index],
		}
	}
	return final
}

func (idx *FlatIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.vectors = append(idx.vectors[:i], idx.vectors[i+1:]...)
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
			return true
		}
	}
	return false
}

func (idx *FlatIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.vectors)
}

func (idx *FlatIndex) Dimension() int {
	return idx.dimension
}

func (idx *FlatIndex) GetDocID(pk string) (uint64, bool) {
	for i, p := range idx.pks {
		if p == pk {
			return uint64(i), true
		}
	}
	return 0, false
}

func (idx *FlatIndex) Close() error {
	return nil
}
