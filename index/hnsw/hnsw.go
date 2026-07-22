package hnsw

import (
	"bufio"
	"container/heap"
	"io"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/persist"
	"github.com/third-apps/go-zvec/types"
)

type neighbor struct {
	id   uint64
	dist float32
}

type neighborMinHeap []neighbor

func (h neighborMinHeap) Len() int            { return len(h) }
func (h neighborMinHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h neighborMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *neighborMinHeap) Push(x interface{}) { *h = append(*h, x.(neighbor)) }
func (h *neighborMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type neighborMaxHeap []neighbor

func (h neighborMaxHeap) Len() int            { return len(h) }
func (h neighborMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h neighborMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *neighborMaxHeap) Push(x interface{}) { *h = append(*h, x.(neighbor)) }
func (h *neighborMaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type HNSWIndex struct {
	mu             sync.RWMutex
	vectors        [][]float32
	pks            []string
	pkToNodeID     map[string]int
	dimension      int
	metricType     types.MetricType
	distFn         metric.DistanceFunc
	m              int
	mMax           int
	mMax0          int
	ef             int
	efConstruction int

	adjList      [][][]uint64
	nodeLevel    []int
	enterPoint   int
	maxLevel     int
	rng          *rand.Rand
	epochPool    sync.Pool
	currentEpoch uint64
	deleted      []bool
	liveCount    int
}

func NewHNSWIndex(dimension int, metricType types.MetricType, m, efConstruction int) *HNSWIndex {
	return &HNSWIndex{
		dimension:      dimension,
		metricType:     metricType,
		distFn:         metric.GetDistanceFunc(metricType),
		m:              m,
		mMax:           m,
		mMax0:          m * 2,
		ef:             300,
		efConstruction: efConstruction,
		vectors:        make([][]float32, 0),
		pks:            make([]string, 0),
		pkToNodeID:     make(map[string]int),
		adjList:        make([][][]uint64, 0),
		nodeLevel:      make([]int, 0),
		enterPoint:     -1,
		maxLevel:       -1,
		rng:            rand.New(rand.NewSource(42)),
		epochPool: sync.Pool{
			New: func() interface{} {
				v := make([]uint64, 0, 65536)
				return &v
			},
		},
	}
}

func (idx *HNSWIndex) SetEF(ef int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if ef > 0 {
		idx.ef = ef
	}
}

func (idx *HNSWIndex) ensureLayerSlots(nodeID uint64, level int) {
	for len(idx.adjList[nodeID]) <= level {
		idx.adjList[nodeID] = append(idx.adjList[nodeID], nil)
	}
}

func (idx *HNSWIndex) Add(vector []float32, pk string) uint64 {
	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	idx.mu.Lock()

	docID := uint64(len(idx.vectors))
	idx.vectors = append(idx.vectors, v)
	idx.pks = append(idx.pks, pk)
	idx.pkToNodeID[pk] = len(idx.vectors) - 1
	idx.adjList = append(idx.adjList, nil)
	idx.deleted = append(idx.deleted, false)
	idx.liveCount++

	level := idx.randomLevel()
	idx.nodeLevel = append(idx.nodeLevel, level)
	idx.ensureLayerSlots(docID, level)

	if idx.enterPoint < 0 {
		idx.enterPoint = int(docID)
		idx.maxLevel = level
		idx.mu.Unlock()
		return docID
	}

	enterPoint := idx.enterPoint
	maxLevel := idx.maxLevel
	idx.mu.Unlock()

	idx.mu.RLock()
	currObj := uint64(enterPoint)
	for lc := maxLevel; lc > level; lc-- {
		result := idx.searchLayer(v, currObj, 1.0, lc)
		if len(result) > 0 {
			currObj = result[0].id
		}
	}

	limit := min(level, maxLevel)
	selectedByLayer := make([][]neighbor, limit+1)
	for lc := limit; lc >= 0; lc-- {
		topCandidates := idx.searchLayer(v, currObj, float64(idx.efConstruction), lc)
		mVal := idx.mMax
		if lc == 0 {
			mVal = idx.mMax0
		}
		selectedByLayer[lc] = selectNeighborsHeuristic(topCandidates, mVal, idx.vectors, v, idx.distFn)
		if len(selectedByLayer[lc]) > 0 {
			currObj = selectedByLayer[lc][0].id
		}
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	for lc := 0; lc <= limit; lc++ {
		for _, n := range selectedByLayer[lc] {
			idx.connectNodes(docID, n.id, lc)
		}
	}
	if level > idx.maxLevel {
		idx.maxLevel = level
		idx.enterPoint = int(docID)
	}
	idx.mu.Unlock()

	return docID
}

func (idx *HNSWIndex) Search(query []float32, topK int) []types.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.searchLocked(query, topK)
}

func (idx *HNSWIndex) searchLocked(query []float32, topK int) []types.SearchResult {
	if idx.enterPoint < 0 || len(idx.vectors) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	currObj := uint64(idx.enterPoint)
	for lc := idx.maxLevel; lc > 0; lc-- {
		result := idx.searchLayer(q, currObj, 1.0, lc)
		if len(result) > 0 {
			currObj = result[0].id
		}
	}

	candidates := idx.searchLayer(q, currObj, float64(idx.ef), 0)

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]types.SearchResult, 0, topK)
	for i := 0; i < len(candidates) && len(results) < topK; i++ {
		n := candidates[i]
		if idx.deleted[n.id] {
			continue
		}
		results = append(results, types.SearchResult{
			DocID: n.id,
			Score: 1.0 / (1.0 + n.dist),
			PK:    idx.pks[n.id],
		})
	}
	return results
}

func (idx *HNSWIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []types.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.enterPoint < 0 || len(idx.vectors) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	maxEf := len(idx.vectors)
	ef := topK * 2
	if ef < idx.ef {
		ef = idx.ef
	}
	if ef > maxEf {
		ef = maxEf
	}

	currObj := uint64(idx.enterPoint)
	for lc := idx.maxLevel; lc > 0; lc-- {
		result := idx.searchLayer(q, currObj, 1.0, lc)
		if len(result) > 0 {
			currObj = result[0].id
		}
	}

	epoch := atomic.AddUint64(&idx.currentEpoch, 1)
	n := len(idx.vectors)
	epochPtr := idx.epochPool.Get().(*[]uint64)
	var visited []uint64
	if cap(*epochPtr) >= n {
		visited = (*epochPtr)[:n]
	} else {
		visited = make([]uint64, n)
	}

	candidates := &neighborMinHeap{}
	heap.Init(candidates)
	resultHeap := &neighborMaxHeap{}
	heap.Init(resultHeap)

	visited[currObj] = epoch
	entryDist := idx.distFn(q, idx.vectors[currObj])
	en := neighbor{id: currObj, dist: entryDist}
	heap.Push(candidates, en)
	heap.Push(resultHeap, en)

	var results []types.SearchResult
	lastEf := 0
	for ef <= maxEf && ef != lastEf {
		lastEf = ef

		idx.expandSearchLayer(q, float64(ef), 0, visited, epoch, candidates, resultHeap)

		results = nil
		sorted := make([]neighbor, resultHeap.Len())
		for i, nb := range *resultHeap {
			sorted[i] = nb
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].dist < sorted[j].dist
		})

		for _, nb := range sorted {
			if idx.deleted[nb.id] {
				continue
			}
			if filterFn(idx.pks[nb.id]) {
				results = append(results, types.SearchResult{
					DocID: nb.id,
					Score: 1.0 / (1.0 + nb.dist),
					PK:    idx.pks[nb.id],
				})
				if len(results) >= topK {
					*epochPtr = visited[:cap(visited)]
					idx.epochPool.Put(epochPtr)
					return results
				}
			}
		}

		if len(results) >= topK {
			break
		}

		ef *= 2
		if ef > maxEf {
			ef = maxEf
		}
	}

	*epochPtr = visited[:cap(visited)]
	idx.epochPool.Put(epochPtr)
	return results
}

func (idx *HNSWIndex) expandSearchLayer(queryVec []float32, ef float64, layer int,
	visited []uint64, epoch uint64, candidates *neighborMinHeap, result *neighborMaxHeap) {

	for candidates.Len() > 0 {
		closest := (*candidates)[0]

		if result.Len() >= int(ef) && closest.dist > (*result)[0].dist {
			break
		}

		heap.Pop(candidates)

		neighbors := idx.getNeighbors(closest.id, layer)

		for _, neighborID := range neighbors {
			if visited[neighborID] == epoch {
				continue
			}
			visited[neighborID] = epoch

			if idx.deleted[neighborID] {
				nnbs := idx.getNeighbors(neighborID, layer)
				for _, nnb := range nnbs {
					if visited[nnb] == epoch {
						continue
					}
					visited[nnb] = epoch
					if idx.deleted[nnb] {
						continue
					}
					dist := idx.distFn(queryVec, idx.vectors[nnb])
					nn := neighbor{id: nnb, dist: dist}
					if result.Len() < int(ef) || dist < (*result)[0].dist {
						heap.Push(candidates, nn)
						heap.Push(result, nn)
						if result.Len() > int(ef) {
							heap.Pop(result)
						}
					}
				}
				continue
			}

			dist := idx.distFn(queryVec, idx.vectors[neighborID])
			nn := neighbor{id: neighborID, dist: dist}

			if result.Len() < int(ef) || dist < (*result)[0].dist {
				heap.Push(candidates, nn)
				heap.Push(result, nn)
				if result.Len() > int(ef) {
					heap.Pop(result)
				}
			}
		}
	}
}

func (idx *HNSWIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	i, ok := idx.pkToNodeID[pk]
	if !ok || idx.deleted[i] {
		return false
	}
	idx.deleted[i] = true
	idx.liveCount--
	delete(idx.pkToNodeID, pk)

	idx.cleanupAdjListForDeletedNode(uint64(i))

	if i == idx.enterPoint {
		newEP := -1
		bestLevel := -1
		for j, l := range idx.nodeLevel {
			if !idx.deleted[j] && l > bestLevel {
				bestLevel = l
				newEP = j
			}
		}
		if newEP < 0 {
			for j := range idx.vectors {
				if !idx.deleted[j] {
					newEP = j
					break
				}
			}
		}
		idx.enterPoint = newEP
		if newEP < 0 {
			idx.maxLevel = -1
		} else {
			for idx.maxLevel >= 0 {
				has := false
				for k, l := range idx.nodeLevel {
					if !idx.deleted[k] && l >= idx.maxLevel {
						has = true
						break
					}
				}
				if !has {
					idx.maxLevel--
				} else {
					break
				}
			}
		}
	}

	return true
}

func (idx *HNSWIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.liveCount
}

func (idx *HNSWIndex) Dimension() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.dimension
}

func (idx *HNSWIndex) MemoryBytes() uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var total uint64
	for _, v := range idx.vectors {
		total += uint64(len(v)) * 4
	}
	for _, pk := range idx.pks {
		total += uint64(len(pk))
	}
	total += uint64(len(idx.deleted))
	for _, layers := range idx.adjList {
		for _, nbs := range layers {
			total += uint64(len(nbs)) * 8
		}
	}
	total += uint64(len(idx.nodeLevel)) * 8
	return total
}

func (idx *HNSWIndex) Save(w io.Writer) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if err := persist.WriteHeader(bw, persist.FileHeader{Magic: persist.MagicNumber, Version: 3, IndexType: persist.IndexTypeHNSW}); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.dimension); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, int(idx.metricType)); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.m); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.mMax); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.mMax0); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.ef); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.efConstruction); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.enterPoint); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.maxLevel); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.liveCount); err != nil {
		return err
	}

	if err := persist.WriteUint64(bw, uint64(len(idx.vectors))); err != nil {
		return err
	}
	for _, v := range idx.vectors {
		if err := persist.WriteFloat32Slice(bw, v); err != nil {
			return err
		}
	}

	if err := persist.WriteUint64(bw, uint64(len(idx.pks))); err != nil {
		return err
	}
	for _, pk := range idx.pks {
		if err := persist.WriteString(bw, pk); err != nil {
			return err
		}
	}

	if err := persist.WriteIntSlice(bw, idx.nodeLevel); err != nil {
		return err
	}

	if err := persist.WriteUint64(bw, uint64(len(idx.adjList))); err != nil {
		return err
	}
	for _, layers := range idx.adjList {
		if err := persist.WriteUint32(bw, uint32(len(layers))); err != nil {
			return err
		}
		for _, nbs := range layers {
			if err := persist.WriteUint64Slice(bw, nbs); err != nil {
				return err
			}
		}
	}

	return persist.WriteBoolSlice(bw, idx.deleted)
}

func (idx *HNSWIndex) Load(r io.Reader) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	br := bufio.NewReader(r)

	h, err := persist.ReadHeader(br)
	if err != nil {
		return err
	}
	if h.IndexType != persist.IndexTypeHNSW {
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
	m, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	mMax, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	mMax0, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	ef, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	efConstruction, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	enterPoint, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	maxLevel, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	liveCount, err := persist.ReadInt(br)
	if err != nil {
		return err
	}

	var vecCount uint64
	if h.Version >= 3 {
		vecCount, err = persist.ReadUint64(br)
	} else {
		vc, e := persist.ReadUint32(br)
		if e != nil {
			return e
		}
		vecCount = uint64(vc)
	}
	if err != nil {
		return err
	}
	vectors := make([][]float32, vecCount)
	for i := range vectors {
		vectors[i], err = persist.ReadFloat32Slice(br)
		if err != nil {
			return err
		}
	}

	var pkCount uint64
	if h.Version >= 3 {
		pkCount, err = persist.ReadUint64(br)
	} else {
		pc, e := persist.ReadUint32(br)
		if e != nil {
			return e
		}
		pkCount = uint64(pc)
	}
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

	nodeLevel, err := persist.ReadIntSlice(br)
	if err != nil {
		return err
	}

	var adjList [][][]uint64
	if h.Version >= 2 {
		var adjCount uint64
		if h.Version >= 3 {
			adjCount, err = persist.ReadUint64(br)
			if err != nil {
				return err
			}
		} else {
			ac, e := persist.ReadUint32(br)
			if e != nil {
				return e
			}
			adjCount = uint64(ac)
		}
		adjList = make([][][]uint64, adjCount)
		for i := range adjList {
			layerCount, err := persist.ReadUint32(br)
			if err != nil {
				return err
			}
			adjList[i] = make([][]uint64, layerCount)
			for l := range adjList[i] {
				adjList[i][l], err = persist.ReadUint64Slice(br)
				if err != nil {
					return err
				}
			}
		}
	} else {
		adjCount, err := persist.ReadUint32(br)
		if err != nil {
			return err
		}
		flatAdj := make([][]uint64, adjCount)
		for i := range flatAdj {
			flatAdj[i], err = persist.ReadUint64Slice(br)
			if err != nil {
				return err
			}
		}
		adjList = make([][][]uint64, adjCount)
		for i, nbs := range flatAdj {
			lvl := 0
			if i < len(nodeLevel) {
				lvl = nodeLevel[i]
			}
			adjList[i] = make([][]uint64, lvl+1)
			if len(adjList[i]) > 0 {
				adjList[i][0] = nbs
			}
		}
	}

	deleted, err := persist.ReadBoolSlice(br)
	if err != nil {
		return err
	}

	idx.dimension = dim
	idx.metricType = types.MetricType(mt)
	idx.distFn = metric.GetDistanceFunc(idx.metricType)
	idx.m = m
	idx.mMax = mMax
	idx.mMax0 = mMax0
	idx.ef = ef
	idx.efConstruction = efConstruction
	idx.enterPoint = enterPoint
	idx.maxLevel = maxLevel
	idx.liveCount = liveCount
	idx.vectors = vectors
	idx.pks = pks
	idx.nodeLevel = nodeLevel
	idx.adjList = adjList
	idx.deleted = deleted
	return nil
}

func (idx *HNSWIndex) Close() error {
	return nil
}

func (idx *HNSWIndex) getNeighbors(nodeID uint64, layer int) []uint64 {
	if int(nodeID) >= len(idx.adjList) {
		return nil
	}
	layers := idx.adjList[nodeID]
	if layer >= len(layers) {
		return nil
	}
	return layers[layer]
}

func (idx *HNSWIndex) searchLayer(queryVec []float32, entryID uint64, ef float64, layer int) []neighbor {
	n := len(idx.vectors)
	epoch := atomic.AddUint64(&idx.currentEpoch, 1)

	epochPtr := idx.epochPool.Get().(*[]uint64)
	var visited []uint64
	if cap(*epochPtr) >= n {
		visited = (*epochPtr)[:n]
	} else {
		visited = make([]uint64, n)
	}
	defer func() {
		*epochPtr = visited[:cap(visited)]
		idx.epochPool.Put(epochPtr)
	}()

	visited[entryID] = epoch

	entryDist := idx.distFn(queryVec, idx.vectors[entryID])
	en := neighbor{id: entryID, dist: entryDist}

	candidates := &neighborMinHeap{en}
	heap.Init(candidates)

	result := &neighborMaxHeap{en}
	heap.Init(result)

	for candidates.Len() > 0 {
		closest := (*candidates)[0]

		if result.Len() >= int(ef) && closest.dist > (*result)[0].dist {
			break
		}

		heap.Pop(candidates)

		neighbors := idx.getNeighbors(closest.id, layer)

		for _, neighborID := range neighbors {
			if visited[neighborID] == epoch {
				continue
			}
			visited[neighborID] = epoch

			if idx.deleted[neighborID] {
				nnbs := idx.getNeighbors(neighborID, layer)
				for _, nnb := range nnbs {
					if visited[nnb] == epoch {
						continue
					}
					visited[nnb] = epoch
					if idx.deleted[nnb] {
						continue
					}
					dist := idx.distFn(queryVec, idx.vectors[nnb])
					nn := neighbor{id: nnb, dist: dist}
					if result.Len() < int(ef) || dist < (*result)[0].dist {
						heap.Push(candidates, nn)
						heap.Push(result, nn)
						if result.Len() > int(ef) {
							heap.Pop(result)
						}
					}
				}
				continue
			}

			dist := idx.distFn(queryVec, idx.vectors[neighborID])
			nn := neighbor{id: neighborID, dist: dist}

			if result.Len() < int(ef) || dist < (*result)[0].dist {
				heap.Push(candidates, nn)
				heap.Push(result, nn)
				if result.Len() > int(ef) {
					heap.Pop(result)
				}
			}
		}
	}

	sorted := make([]neighbor, result.Len())
	for i, n := range *result {
		sorted[i] = n
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].dist < sorted[j].dist
	})
	return sorted
}

func (idx *HNSWIndex) cleanupAdjListForDeletedNode(nodeID uint64) {
	for nodeIdx, layers := range idx.adjList {
		for layer, nbs := range layers {
			filtered := make([]uint64, 0, len(nbs))
			for _, nb := range nbs {
				if nb != nodeID {
					filtered = append(filtered, nb)
				}
			}
			if len(filtered) != len(nbs) {
				idx.adjList[nodeIdx][layer] = filtered
			}
		}
	}
}

func (idx *HNSWIndex) connectNodes(a, b uint64, layer int) {
	idx.addNeighbor(a, b, layer)
	idx.addNeighbor(b, a, layer)
}

func (idx *HNSWIndex) addNeighbor(node, neighborID uint64, layer int) {
	idx.ensureLayerSlots(node, layer)
	nbs := idx.adjList[node][layer]
	for _, nb := range nbs {
		if nb == neighborID {
			return
		}
	}

	maxConns := idx.mMax
	if layer == 0 {
		maxConns = idx.mMax0
	}

	nbs = append(nbs, neighborID)
	if len(nbs) > maxConns {
		nbs = pruneNeighbors(node, nbs, idx.vectors, maxConns, idx.distFn)
	}
	idx.adjList[node][layer] = nbs
}

func (idx *HNSWIndex) randomLevel() int {
	if idx.m <= 1 {
		return 0
	}
	ml := 1.0 / math.Log(float64(idx.m))
	level := int(math.Floor(-math.Log(idx.rng.Float64()) * ml))
	if level < 0 {
		level = 0
	}
	return level
}

func selectNeighborsSimple(candidates []neighbor, m int) []neighbor {
	if len(candidates) <= m {
		return candidates
	}
	sorted := make([]neighbor, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].dist < sorted[j].dist
	})
	return sorted[:m]
}

func selectNeighborsHeuristic(candidates []neighbor, m int, allVectors [][]float32, queryVec []float32, distFn metric.DistanceFunc) []neighbor {
	if len(candidates) <= m {
		return candidates
	}
	sorted := make([]neighbor, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].dist < sorted[j].dist
	})

	var selected []neighbor
	for _, c := range sorted {
		if len(selected) >= m {
			break
		}
		good := true
		for _, s := range selected {
			if int(c.id) < len(allVectors) && int(s.id) < len(allVectors) {
				distToSelected := distFn(allVectors[c.id], allVectors[s.id])
				if distToSelected < c.dist {
					good = false
					break
				}
			}
		}
		if good {
			selected = append(selected, c)
		}
	}

	if len(selected) < m {
		for _, c := range sorted {
			if len(selected) >= m {
				break
			}
			found := false
			for _, s := range selected {
				if s.id == c.id {
					found = true
					break
				}
			}
			if !found {
				selected = append(selected, c)
			}
		}
	}

	return selected
}

func pruneNeighbors(nodeID uint64, neighbors []uint64, allVectors [][]float32, maxCount int, distFn metric.DistanceFunc) []uint64 {
	type distIdx struct {
		id   uint64
		dist float32
	}
	dists := make([]distIdx, len(neighbors))
	for i, nb := range neighbors {
		dists[i] = distIdx{id: nb, dist: distFn(allVectors[nodeID], allVectors[nb])}
	}
	sort.Slice(dists, func(i, j int) bool {
		return dists[i].dist < dists[j].dist
	})
	if len(dists) > maxCount {
		dists = dists[:maxCount]
	}
	result := make([]uint64, len(dists))
	for i, d := range dists {
		result[i] = d.id
	}
	return result
}
