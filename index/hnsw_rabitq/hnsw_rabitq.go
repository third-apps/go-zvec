package hnsw_rabitq

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/quantizer"
	"github.com/third-apps/go-zvec/types"
)

type neighbor struct {
	id   uint64
	dist float32
}

type minHeap []neighbor

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(neighbor)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type maxHeap []neighbor

func (h maxHeap) Len() int            { return len(h) }
func (h maxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h maxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x interface{}) { *h = append(*h, x.(neighbor)) }
func (h *maxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type HNSWRabitqIndex struct {
	mu             sync.RWMutex
	codes          [][]byte
	pks            []string
	rawVectors     [][]float32
	dimension      int
	metricType     types.MetricType
	distFn         metric.DistanceFunc
	m              int
	mMax           int
	mMax0          int
	ef             int
	efConstruction int
	adjList        [][]uint64
	nodeLevel      []int
	enterPoint     int
	maxLevel       int
	rng            *rand.Rand
	codeSize       int
	quantizer      *quantizer.Int4Quantizer
}

func NewHNSWRabitqIndex(dimension int, metricType types.MetricType, m, efConstruction int) *HNSWRabitqIndex {
	return &HNSWRabitqIndex{
		dimension:      dimension,
		metricType:     metricType,
		distFn:         metric.GetDistanceFunc(metricType),
		m:              m,
		mMax:           m,
		mMax0:          m * 2,
		ef:             300,
		efConstruction: efConstruction,
		pks:            make([]string, 0),
		codes:          make([][]byte, 0),
		rawVectors:     make([][]float32, 0),
		adjList:        make([][]uint64, 0),
		nodeLevel:      make([]int, 0),
		enterPoint:     -1,
		maxLevel:       -1,
		rng:            rand.New(rand.NewSource(42)),
		codeSize:       (dimension + 1) / 2,
		quantizer:      quantizer.NewInt4Quantizer(dimension, true),
	}
}

func (idx *HNSWRabitqIndex) SetEF(ef int) {
	if ef > 0 {
		idx.ef = ef
	}
}

func (idx *HNSWRabitqIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	code := idx.quantizer.Encode(v, nil)

	docID := uint64(len(idx.codes))
	idx.codes = append(idx.codes, code)
	idx.pks = append(idx.pks, pk)
	idx.rawVectors = append(idx.rawVectors, v)
	idx.adjList = append(idx.adjList, []uint64{})

	level := idx.randomLevel()
	idx.nodeLevel = append(idx.nodeLevel, level)

	if idx.enterPoint < 0 {
		idx.enterPoint = int(docID)
		idx.maxLevel = level
		return docID
	}

	currObj := uint64(idx.enterPoint)
	for lc := idx.maxLevel; lc > level; lc-- {
		currObj = idx.searchLayer(v, currObj, 1.0, lc)[0].id
	}

	for lc := min(level, idx.maxLevel); lc >= 0; lc-- {
		topCandidates := idx.searchLayer(v, currObj, float64(idx.efConstruction), lc)
		mVal := idx.mMax
		if lc == 0 {
			mVal = idx.mMax0
		}
		selected := selectNeighborsSimple(topCandidates, mVal)
		for _, n := range selected {
			idx.connectNodes(docID, n.id, lc)
		}
		if len(selected) > 0 {
			currObj = selected[0].id
		}
	}

	if level > idx.maxLevel {
		idx.maxLevel = level
		idx.enterPoint = int(docID)
	}

	return docID
}

func (idx *HNSWRabitqIndex) Search(query []float32, topK int) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.searchLocked(query, topK)
}

func (idx *HNSWRabitqIndex) searchLocked(query []float32, topK int) []flat.SearchResult {
	if idx.enterPoint < 0 || len(idx.codes) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	currObj := uint64(idx.enterPoint)
	for lc := idx.maxLevel; lc > 0; lc-- {
		currObj = idx.searchLayer(q, currObj, 1.0, lc)[0].id
	}

	candidates := idx.searchLayer(q, currObj, float64(idx.ef), 0)

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]flat.SearchResult, topK)
	for i := 0; i < topK; i++ {
		n := candidates[i]
		results[i] = flat.SearchResult{
			DocID: n.id,
			Score: 1.0 / (1.0 + n.dist),
			PK:    idx.pks[n.id],
		}
	}
	return results
}

func (idx *HNSWRabitqIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	all := idx.searchLocked(query, len(idx.codes))
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

func (idx *HNSWRabitqIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.codes = append(idx.codes[:i], idx.codes[i+1:]...)
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
			idx.rawVectors = append(idx.rawVectors[:i], idx.rawVectors[i+1:]...)
			idx.adjList = append(idx.adjList[:i], idx.adjList[i+1:]...)
			idx.nodeLevel = append(idx.nodeLevel[:i], idx.nodeLevel[i+1:]...)

			for j := range idx.adjList {
				newList := make([]uint64, 0, len(idx.adjList[j]))
				for _, nb := range idx.adjList[j] {
					if nb == uint64(i) {
						continue
					}
					if nb > uint64(i) {
						newList = append(newList, nb-1)
					} else {
						newList = append(newList, nb)
					}
				}
				idx.adjList[j] = newList
			}

			if idx.enterPoint == i {
				if len(idx.codes) > 0 {
					idx.enterPoint = 0
				} else {
					idx.enterPoint = -1
					idx.maxLevel = -1
				}
			} else if idx.enterPoint > i {
				idx.enterPoint--
			}

			if idx.maxLevel >= len(idx.nodeLevel) {
				idx.maxLevel = len(idx.nodeLevel) - 1
			}
			for idx.maxLevel >= 0 {
				has := false
				for _, l := range idx.nodeLevel {
					if l >= idx.maxLevel {
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
			return true
		}
	}
	return false
}

func (idx *HNSWRabitqIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.codes)
}

func (idx *HNSWRabitqIndex) Dimension() int {
	return idx.dimension
}

func (idx *HNSWRabitqIndex) Close() error {
	return nil
}

func (idx *HNSWRabitqIndex) decodeInt4(code []byte) []float32 {
	vec := make([]float32, idx.dimension)
	for j := 0; j < idx.dimension; j++ {
		var nibble uint8
		if j%2 == 0 {
			nibble = code[j/2] >> 4
		} else {
			nibble = code[j/2] & 0x0F
		}
		if nibble&0x08 != 0 {
			nibble |= 0xF0
		}
		vec[j] = float32(int8(nibble)) / 7.0
	}
	return vec
}

func (idx *HNSWRabitqIndex) approximateDistance(queryVec []float32, code []byte) float32 {
	decoded := idx.decodeInt4(code)
	return idx.distFn(queryVec, decoded)
}

func (idx *HNSWRabitqIndex) searchLayer(queryVec []float32, entryID uint64, ef float64, layer int) []neighbor {
	visited := make(map[uint64]struct{})
	var candidates minHeap
	var farthest maxHeap

	entryDist := idx.approximateDistance(queryVec, idx.codes[entryID])
	en := neighbor{id: entryID, dist: entryDist}

	heap.Push(&candidates, en)
	heap.Push(&farthest, en)
	visited[entryID] = struct{}{}

	for candidates.Len() > 0 {
		closest := candidates[0]
		farthestDist := farthest[0].dist

		if closest.dist > farthestDist {
			break
		}

		cn := heap.Pop(&candidates).(neighbor)

		for _, neighborID := range idx.adjList[cn.id] {
			if _, seen := visited[neighborID]; seen {
				continue
			}
			visited[neighborID] = struct{}{}

			dist := idx.approximateDistance(queryVec, idx.codes[neighborID])
			nn := neighbor{id: neighborID, dist: dist}

			if farthest.Len() < int(ef) || dist < farthest[0].dist {
				heap.Push(&candidates, nn)
				heap.Push(&farthest, nn)
				if farthest.Len() > int(ef) {
					heap.Pop(&farthest)
				}
			}
		}
	}

	result := make([]neighbor, farthest.Len())
	for i, n := range farthest {
		result[i] = n
	}
	return result
}

func (idx *HNSWRabitqIndex) connectNodes(a, b uint64, layer int) {
	idx.addNeighbor(a, b, layer)
	idx.addNeighbor(b, a, layer)
}

func (idx *HNSWRabitqIndex) addNeighbor(node, neighborID uint64, layer int) {
	nbs := idx.adjList[node]
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
		nbs = pruneNeighbors(node, nbs, idx.rawVectors, maxConns, idx.distFn)
	}
	idx.adjList[node] = nbs
}

func (idx *HNSWRabitqIndex) randomLevel() int {
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
