package flat

import (
	"bufio"
	"container/heap"
	"io"
	"sync"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/persist"
	"github.com/third-apps/go-zvec/types"
)

type SearchResult = types.SearchResult

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
	pkToDocID  map[string]int
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
		pkToDocID:  make(map[string]int),
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
	idx.pkToDocID[pk] = idx.count
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

	i, ok := idx.pkToDocID[pk]
	if !ok || idx.deleted[i] {
		return false
	}
	idx.deleted[i] = true
	idx.liveCount--
	delete(idx.pkToDocID, pk)

	if idx.count > 64 && idx.liveCount < idx.count/2 {
		idx.compact()
	}

	return true
}

func (idx *FlatIndex) compact() {
	dim := idx.dimension
	newData := make([]float32, 0, idx.liveCount*dim)
	newPks := make([]string, 0, idx.liveCount)
	newDeleted := make([]bool, 0, idx.liveCount)
	newPkToDocID := make(map[string]int, idx.liveCount)

	for i := 0; i < idx.count; i++ {
		if !idx.deleted[i] {
			offset := i * dim
			newData = append(newData, idx.data[offset:offset+dim]...)
			newPkToDocID[idx.pks[i]] = len(newPks)
			newPks = append(newPks, idx.pks[i])
			newDeleted = append(newDeleted, false)
		}
	}

	idx.data = newData
	idx.pks = newPks
	idx.pkToDocID = newPkToDocID
	idx.deleted = newDeleted
	idx.count = idx.liveCount
}

func (idx *FlatIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.liveCount
}

func (idx *FlatIndex) Dimension() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.dimension
}

func (idx *FlatIndex) GetDocID(pk string) (uint64, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	i, ok := idx.pkToDocID[pk]
	if !ok {
		return 0, false
	}
	return uint64(i), true
}

func (idx *FlatIndex) MemoryBytes() uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var total uint64
	total += uint64(len(idx.data)) * 4
	total += uint64(len(idx.deleted))
	for _, pk := range idx.pks {
		total += uint64(len(pk))
	}
	return total
}

func (idx *FlatIndex) Save(w io.Writer) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if err := persist.WriteHeader(bw, persist.FileHeader{Magic: persist.MagicNumber, Version: 1, IndexType: persist.IndexTypeFlat}); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.dimension); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, int(idx.metricType)); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.count); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.liveCount); err != nil {
		return err
	}
	if err := persist.WriteFloat32Slice(bw, idx.data); err != nil {
		return err
	}
	if err := persist.WriteUint32(bw, uint32(len(idx.pks))); err != nil {
		return err
	}
	for _, pk := range idx.pks {
		if err := persist.WriteString(bw, pk); err != nil {
			return err
		}
	}
	return persist.WriteBoolSlice(bw, idx.deleted)
}

func (idx *FlatIndex) Load(r io.Reader) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	br := bufio.NewReader(r)

	h, err := persist.ReadHeader(br)
	if err != nil {
		return err
	}
	if h.IndexType != persist.IndexTypeFlat {
		return io.ErrUnexpectedEOF
	}

	dim, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	mt, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	count, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	liveCount, err := persist.ReadInt(br)
	if err != nil {
		return err
	}

	data, err := persist.ReadFloat32Slice(br)
	if err != nil {
		return err
	}

	pkCount, err := persist.ReadUint32(br)
	if err != nil {
		return err
	}
	pks := make([]string, pkCount)
	for i := range pks {
		pks[i], err = persist.ReadString(br)
		if err != nil {
			return err
		}
	}

	deleted, err := persist.ReadBoolSlice(br)
	if err != nil {
		return err
	}

	idx.dimension = dim
	idx.metricType = types.MetricType(mt)
	idx.distFn = metric.GetDistanceFunc(idx.metricType)
	idx.count = count
	idx.liveCount = liveCount
	idx.data = data
	idx.pks = pks
	idx.deleted = deleted
	return nil
}

func (idx *FlatIndex) Close() error {
	return nil
}
