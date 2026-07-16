package vamana

import (
	"math/rand"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

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
	idx.entryPoint = int(docID)

	if idx.saturateGraph {
		idx.ensureDegree(int(docID))
	}

	return docID
}

func (idx *VamanaIndex) Search(query []float32, topK int) []flat.SearchResult {
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

	candidates := idx.greedySearch(q, idx.entryPoint, idx.searchListSize)

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

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

	sort.Slice(all, func(i, j int) bool {
		return all[i].dist < all[j].dist
	})

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

type vNeighbor struct {
	id   int
	dist float32
}

func (idx *VamanaIndex) greedySearch(query []float32, start int, L int) []vNeighbor {
	visited := make(map[int]struct{})
	var candidates []vNeighbor
	var results []vNeighbor

	startDist := idx.distFn(query, idx.docs[start])
	startN := vNeighbor{id: start, dist: startDist}
	candidates = append(candidates, startN)
	results = append(results, startN)
	visited[start] = struct{}{}

	for len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].dist < candidates[j].dist
		})

		closest := candidates[0]
		farthest := results[len(results)-1]

		if closest.dist > farthest.dist {
			break
		}

		candidates = candidates[1:]

		for _, nb := range idx.graph[closest.id] {
			if _, seen := visited[nb]; seen {
				continue
			}
			visited[nb] = struct{}{}
			dist := idx.distFn(query, idx.docs[nb])
			nn := vNeighbor{id: nb, dist: dist}

			if len(candidates) < L || dist < candidates[len(candidates)-1].dist {
				candidates = append(candidates, nn)
				results = append(results, nn)
				sort.Slice(results, func(i, j int) bool {
					return results[i].dist < results[j].dist
				})
				if len(results) > L {
					results = results[:L]
				}
			}
		}
	}

	return results
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

func (idx *VamanaIndex) ensureDegree(nodeID int) {
	currentDegree := len(idx.graph[nodeID])
	if currentDegree >= idx.maxDegree {
		return
	}

	type candidate struct {
		id   int
		dist float32
	}
	var candidates []candidate
	for i := range idx.docs {
		if i == nodeID {
			continue
		}
		alreadyConnected := false
		for _, nb := range idx.graph[nodeID] {
			if nb == i {
				alreadyConnected = true
				break
			}
		}
		if alreadyConnected {
			continue
		}
		dist := idx.distFn(idx.docs[nodeID], idx.docs[i])
		candidates = append(candidates, candidate{id: i, dist: dist})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	needed := idx.maxDegree - currentDegree
	if needed > len(candidates) {
		needed = len(candidates)
	}

	for i := 0; i < needed; i++ {
		idx.addUndirectedEdge(nodeID, candidates[i].id)
	}
}
