package hnsw

import (
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type neighbor struct {
	id   uint64
	dist float32
}

type HNSWIndex struct {
	mu             sync.RWMutex
	vectors        [][]float32
	pks            []string
	dimension      int
	metricType     types.MetricType
	distFn         metric.DistanceFunc
	m              int
	mMax           int
	mMax0          int
	ef             int
	efConstruction int

	adjList    [][]uint64
	nodeLevel  []int
	enterPoint int
	maxLevel   int
	rng        *rand.Rand
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
		adjList:        make([][]uint64, 0),
		nodeLevel:      make([]int, 0),
		enterPoint:     -1,
		maxLevel:       -1,
		rng:            rand.New(rand.NewSource(42)),
	}
}

func (idx *HNSWIndex) SetEF(ef int) {
	if ef > 0 {
		idx.ef = ef
	}
}

func (idx *HNSWIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	docID := uint64(len(idx.vectors))
	idx.vectors = append(idx.vectors, v)
	idx.pks = append(idx.pks, pk)
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
		result := idx.searchLayer(v, currObj, 1.0, lc)
		if len(result) > 0 {
			currObj = result[0].id
		}
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

func (idx *HNSWIndex) Search(query []float32, topK int) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.searchLocked(query, topK)
}

func (idx *HNSWIndex) searchLocked(query []float32, topK int) []flat.SearchResult {
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

func (idx *HNSWIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	all := idx.searchLocked(query, len(idx.vectors))
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

func (idx *HNSWIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.vectors = append(idx.vectors[:i], idx.vectors[i+1:]...)
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
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
				if len(idx.vectors) > 0 {
					idx.enterPoint = 0
				} else {
					idx.enterPoint = -1
					idx.maxLevel = -1
				}
			} else if idx.enterPoint > i {
				idx.enterPoint--
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

func (idx *HNSWIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.vectors)
}

func (idx *HNSWIndex) Dimension() int {
	return idx.dimension
}

func (idx *HNSWIndex) Close() error {
	return nil
}

func (idx *HNSWIndex) searchLayer(queryVec []float32, entryID uint64, ef float64, layer int) []neighbor {
	n := len(idx.vectors)
	visited := make([]byte, n)
	visited[entryID] = 1

	entryDist := idx.distFn(queryVec, idx.vectors[entryID])

	pool := make([]neighbor, 0, int(ef)+1)
	pool = append(pool, neighbor{id: entryID, dist: entryDist})

	candidateSet := make([]neighbor, 0, int(ef)+1)
	candidateSet = append(candidateSet, neighbor{id: entryID, dist: entryDist})

	for len(candidateSet) > 0 {
		closest := candidateSet[0]
		copy(candidateSet, candidateSet[1:])
		candidateSet = candidateSet[:len(candidateSet)-1]

		farthestDist := pool[len(pool)-1].dist
		if closest.dist > farthestDist && len(pool) >= int(ef) {
			break
		}

		for _, neighborID := range idx.adjList[closest.id] {
			if visited[neighborID] != 0 {
				continue
			}
			visited[neighborID] = 1

			dist := idx.distFn(queryVec, idx.vectors[neighborID])
			nn := neighbor{id: neighborID, dist: dist}

			if len(pool) < int(ef) || dist < pool[len(pool)-1].dist {
				pool = insertSorted(pool, nn)
				if len(pool) > int(ef) {
					pool = pool[:int(ef)]
				}
				candidateSet = insertSorted(candidateSet, nn)
			}
		}
	}

	return pool
}

func insertSorted(s []neighbor, n neighbor) []neighbor {
	i := sort.Search(len(s), func(j int) bool { return s[j].dist > n.dist })
	s = append(s, neighbor{})
	copy(s[i+1:], s[i:])
	s[i] = n
	return s
}

func (idx *HNSWIndex) connectNodes(a, b uint64, layer int) {
	idx.addNeighbor(a, b, layer)
	idx.addNeighbor(b, a, layer)
}

func (idx *HNSWIndex) addNeighbor(node, neighborID uint64, layer int) {
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
		nbs = pruneNeighbors(node, nbs, idx.vectors, maxConns, idx.distFn)
	}
	idx.adjList[node] = nbs
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
