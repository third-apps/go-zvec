package vamana

import (
	"container/heap"
	"math/rand"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type vNeighbor struct {
	id   int
	dist float32
}

type vMinHeap []vNeighbor

func (h vMinHeap) Len() int            { return len(h) }
func (h vMinHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h vMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *vMinHeap) Push(x interface{}) { *h = append(*h, x.(vNeighbor)) }
func (h *vMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type vMaxHeap []vNeighbor

func (h vMaxHeap) Len() int            { return len(h) }
func (h vMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h vMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *vMaxHeap) Push(x interface{}) { *h = append(*h, x.(vNeighbor)) }
func (h *vMaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type VamanaIndex struct {
	mu             sync.RWMutex
	dimension      int
	metricType     types.MetricType
	distFn         metric.DistanceFunc
	maxDegree      int
	searchListSize int
	alpha          float32
	saturateGraph  bool

	docs       [][]float32
	pks        []string
	graph      [][]int
	rng        *rand.Rand
	entryPoint int

	visitedPool sync.Pool
}

func NewVamanaIndex(dimension int, metricType types.MetricType,
	maxDegree, searchListSize int, alpha float32, saturateGraph bool) *VamanaIndex {
	return &VamanaIndex{
		dimension:      dimension,
		metricType:     metricType,
		distFn:         metric.GetDistanceFunc(metricType),
		maxDegree:      maxDegree,
		searchListSize: searchListSize,
		alpha:          alpha,
		saturateGraph:  saturateGraph,
		docs:           make([][]float32, 0),
		pks:            make([]string, 0),
		graph:          make([][]int, 0),
		rng:            rand.New(rand.NewSource(42)),
		entryPoint:     -1,
		visitedPool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, 65536)
				return &b
			},
		},
	}
}

func (idx *VamanaIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	docID := uint64(len(idx.docs))
	idx.docs = append(idx.docs, v)
	idx.pks = append(idx.pks, pk)
	idx.graph = append(idx.graph, []int{})

	if idx.entryPoint < 0 {
		idx.entryPoint = 0
		return docID
	}

	candidates := idx.greedySearch(v, idx.entryPoint, idx.searchListSize)
	idx.pruneAndAdd(int(docID), candidates)

	if idx.saturateGraph {
		idx.ensureDegreeFromCandidates(int(docID), candidates)
	}

	return docID
}

func (idx *VamanaIndex) Search(query []float32, topK int) []flat.SearchResult {
	return idx.SearchWithListSize(query, topK, 0)
}

func (idx *VamanaIndex) SearchWithListSize(query []float32, topK, searchListSize int) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.docs) == 0 || idx.entryPoint < 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	L := idx.searchListSize
	if searchListSize > 0 {
		L = searchListSize
	}
	candidates := idx.greedySearch(q, idx.entryPoint, L)

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]flat.SearchResult, topK)
	for i := 0; i < topK; i++ {
		c := candidates[i]
		results[i] = flat.SearchResult{
			DocID: uint64(c.id),
			Score: 1.0 / (1.0 + c.dist),
			PK:    idx.pks[c.id],
		}
	}
	return results
}

func (idx *VamanaIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.docs) == 0 || idx.entryPoint < 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	all := idx.greedySearch(q, idx.entryPoint, len(idx.docs))

	var results []flat.SearchResult
	for _, c := range all {
		if filterFn(idx.pks[c.id]) {
			results = append(results, flat.SearchResult{
				DocID: uint64(c.id),
				Score: 1.0 / (1.0 + c.dist),
				PK:    idx.pks[c.id],
			})
			if len(results) >= topK {
				break
			}
		}
	}
	return results
}

func (idx *VamanaIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.docs = append(idx.docs[:i], idx.docs[i+1:]...)
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
			idx.graph = append(idx.graph[:i], idx.graph[i+1:]...)
			for j := range idx.graph {
				for k := len(idx.graph[j]) - 1; k >= 0; k-- {
					if idx.graph[j][k] == i || idx.graph[j][k] > i {
						if idx.graph[j][k] > i {
							idx.graph[j][k]--
						} else {
							idx.graph[j] = append(idx.graph[j][:k], idx.graph[j][k+1:]...)
						}
					}
				}
			}
			if idx.entryPoint == i {
				if len(idx.docs) > 0 {
					idx.entryPoint = 0
				} else {
					idx.entryPoint = -1
				}
			} else if idx.entryPoint > i {
				idx.entryPoint--
			}
			return true
		}
	}
	return false
}

func (idx *VamanaIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.docs)
}

func (idx *VamanaIndex) Dimension() int {
	return idx.dimension
}

func (idx *VamanaIndex) Close() error {
	return nil
}

func (idx *VamanaIndex) greedySearch(query []float32, start int, L int) []vNeighbor {
	n := len(idx.docs)
	visitedPtr := idx.visitedPool.Get().(*[]byte)
	var visited []byte
	if cap(*visitedPtr) >= n {
		visited = (*visitedPtr)[:n]
		for i := range visited {
			visited[i] = 0
		}
	} else {
		visited = make([]byte, n)
	}
	defer func() {
		*visitedPtr = visited[:cap(visited)]
		idx.visitedPool.Put(visitedPtr)
	}()

	visited[start] = 1

	startDist := idx.distFn(query, idx.docs[start])
	startN := vNeighbor{id: start, dist: startDist}

	candidates := &vMinHeap{startN}
	heap.Init(candidates)

	result := &vMaxHeap{startN}
	heap.Init(result)

	for candidates.Len() > 0 {
		closest := (*candidates)[0]

		if result.Len() >= L && closest.dist > (*result)[0].dist {
			break
		}

		heap.Pop(candidates)

		for _, nb := range idx.graph[closest.id] {
			if visited[nb] != 0 {
				continue
			}
			visited[nb] = 1
			dist := idx.distFn(query, idx.docs[nb])
			nn := vNeighbor{id: nb, dist: dist}

			if result.Len() < L || dist < (*result)[0].dist {
				heap.Push(candidates, nn)
				heap.Push(result, nn)
				if result.Len() > L {
					heap.Pop(result)
				}
			}
		}
	}

	sorted := make([]vNeighbor, result.Len())
	for i, n := range *result {
		sorted[i] = n
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].dist < sorted[j].dist
	})
	return sorted
}

func (idx *VamanaIndex) pruneAndAdd(newID int, candidates []vNeighbor) {
	maxR := idx.maxDegree

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	var pruned []vNeighbor
	for _, c := range candidates {
		keep := true
		for _, p := range pruned {
			distPC := idx.distFn(idx.docs[p.id], idx.docs[c.id])
			if idx.alpha*distPC <= c.dist {
				keep = false
				break
			}
		}
		if keep {
			pruned = append(pruned, c)
			if len(pruned) >= maxR {
				break
			}
		}
	}

	for _, p := range pruned {
		idx.addUndirectedEdge(newID, p.id)
	}

	for _, p := range pruned {
		if len(idx.graph[p.id]) > maxR {
			idx.trimNeighbors(p.id, maxR)
		}
	}
}

func (idx *VamanaIndex) addUndirectedEdge(a, b int) {
	for _, nb := range idx.graph[a] {
		if nb == b {
			return
		}
	}
	idx.graph[a] = append(idx.graph[a], b)
	idx.graph[b] = append(idx.graph[b], a)
}

func (idx *VamanaIndex) trimNeighbors(nodeID int, maxDegree int) {
	nbs := idx.graph[nodeID]
	neighbors := make([]vNeighbor, len(nbs))
	for i, nb := range nbs {
		neighbors[i] = vNeighbor{id: nb, dist: idx.distFn(idx.docs[nodeID], idx.docs[nb])}
	}
	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].dist < neighbors[j].dist
	})

	var pruned []vNeighbor
	for _, c := range neighbors {
		keep := true
		for _, p := range pruned {
			distPC := idx.distFn(idx.docs[p.id], idx.docs[c.id])
			if idx.alpha*distPC <= c.dist {
				keep = false
				break
			}
		}
		if keep {
			pruned = append(pruned, c)
			if len(pruned) >= maxDegree {
				break
			}
		}
	}

	newNbs := make([]int, len(pruned))
	for i, p := range pruned {
		newNbs[i] = p.id
	}
	idx.graph[nodeID] = newNbs

	stillConnected := make(map[int]struct{})
	for _, p := range pruned {
		stillConnected[p.id] = struct{}{}
	}
	for _, nb := range nbs {
		if _, ok := stillConnected[nb]; !ok {
			for j := len(idx.graph[nb]) - 1; j >= 0; j-- {
				if idx.graph[nb][j] == nodeID {
					idx.graph[nb] = append(idx.graph[nb][:j], idx.graph[nb][j+1:]...)
					break
				}
			}
		}
	}
}

func (idx *VamanaIndex) ensureDegreeFromCandidates(nodeID int, candidates []vNeighbor) {
	currentDegree := len(idx.graph[nodeID])
	if currentDegree >= idx.maxDegree {
		return
	}


	for _, c := range candidates {
		if c.id == nodeID {
			continue
		}
		alreadyConnected := false
		for _, nb := range idx.graph[nodeID] {
			if nb == c.id {
				alreadyConnected = true
				break
			}
		}
		if alreadyConnected {
			continue
		}
		idx.addUndirectedEdge(nodeID, c.id)
		if len(idx.graph[nodeID]) >= idx.maxDegree {
			break
		}
	}
}
