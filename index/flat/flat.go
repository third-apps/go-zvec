package flat

import (
	"container/heap"
	"sync"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type SearchResult struct {
	DocID uint64
	Score float32
	PK    string
}

type flatMaxHeapItem struct {
	dist  float32
	index int
}

type flatMaxHeap []flatMaxHeapItem

func (h flatMaxHeap) Len() int            { return len(h) }
func (h flatMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h flatMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *flatMaxHeap) Push(x interface{}) { *h = append(*h, x.(flatMaxHeapItem)) }
func (h *flatMaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type FlatIndex struct {
	mu         sync.RWMutex
	data       []float32
	pks        []string
	count      int
	liveCount  int
	dimension  int
	metricType types.MetricType
	distFn     metric.DistanceFunc
	deleted    []bool
}

func NewFlatIndex(dimension int, metricType types.MetricType) *FlatIndex {
	return &FlatIndex{
		dimension:  dimension,
		metricType: metricType,
		distFn:     metric.GetDistanceFunc(metricType),
		data:       make([]float32, 0, 1024*dimension),
		pks:        make([]string, 0, 1024),
	}
}

func (idx *FlatIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := uint64(idx.count)
	offset := len(idx.data)
	needed := offset + idx.dimension
	if cap(idx.data) < needed {
		newCap := cap(idx.data) * 2
		if newCap < needed {
			newCap = needed
		}
		newData := make([]float32, len(idx.data), newCap)
		copy(newData, idx.data)
		idx.data = newData
	}
	idx.data = idx.data[:needed]
	copy(idx.data[offset:], vector[:idx.dimension])

	if idx.metricType == types.MetricTypeCosine {
		metric.NormalizeInPlace(idx.data[offset : offset+idx.dimension])
	}

	idx.pks = append(idx.pks, pk)
	idx.deleted = append(idx.deleted, false)
	idx.count++
	idx.liveCount++
	return docID
}

func (idx *FlatIndex) Search(query []float32, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.count == 0 {
		return nil
	}

	q := make([]float32, idx.dimension)
	copy(q, query[:idx.dimension])
	if idx.metricType == types.MetricTypeCosine {
		metric.NormalizeInPlace(q)
	}

	if topK > idx.count {
		topK = idx.count
	}

	h := make(flatMaxHeap, 0, topK)
	dim := idx.dimension
	for i := 0; i < idx.count; i++ {
		if idx.deleted[i] {
			continue
		}
		offset := i * dim
		d := idx.distFn(q, idx.data[offset:offset+dim])
		if h.Len() < topK {
			heap.Push(&h, flatMaxHeapItem{dist: d, index: i})
		} else if d < h[0].dist {
			h[0] = flatMaxHeapItem{dist: d, index: i}
			heap.Fix(&h, 0)
		}
	}

	final := make([]SearchResult, h.Len())
	items := make([]flatMaxHeapItem, h.Len())
	for h.Len() > 0 {
		item := heap.Pop(&h).(flatMaxHeapItem)
		items[h.Len()] = item
	}
	for i, item := range items {
		final[i] = SearchResult{
			DocID: uint64(item.index),
			Score: 1.0 / (1.0 + item.dist),
			PK:    idx.pks[item.index],
		}
	}
	return final
}

func (idx *FlatIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []SearchResult {

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	q := make([]float32, idx.dimension)
	copy(q, query[:idx.dimension])
	if idx.metricType == types.MetricTypeCosine {
		metric.NormalizeInPlace(q)
	}

	h := make(flatMaxHeap, 0, topK)
	dim := idx.dimension
	for i := 0; i < idx.count; i++ {
		if idx.deleted[i] || !filterFn(idx.pks[i]) {
			continue
		}
		offset := i * dim
		d := idx.distFn(q, idx.data[offset:offset+dim])
		if h.Len() < topK {
			heap.Push(&h, flatMaxHeapItem{dist: d, index: i})
		} else if d < h[0].dist {
			h[0] = flatMaxHeapItem{dist: d, index: i}
			heap.Fix(&h, 0)
		}
	}

	final := make([]SearchResult, h.Len())
	items := make([]flatMaxHeapItem, h.Len())
	for h.Len() > 0 {
		item := heap.Pop(&h).(flatMaxHeapItem)
		items[h.Len()] = item
	}
	for i, item := range items {
		final[i] = SearchResult{
			DocID: uint64(item.index),
			Score: 1.0 / (1.0 + item.dist),
			PK:    idx.pks[item.index],
		}
	}
	return final
}

func (idx *FlatIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk && !idx.deleted[i] {
			idx.deleted[i] = true
			idx.liveCount--
			return true
		}
	}
	return false
}

func (idx *FlatIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.liveCount
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
