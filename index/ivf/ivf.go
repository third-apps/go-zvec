package ivf

import (
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

type IVFIndex struct {
	mu         sync.RWMutex
	dimension  int
	metricType types.MetricType
	distFn     metric.DistanceFunc
	nList      int
	nIters     int
	nprobe     int
	useSOAR    bool

	centroids   [][]float32
	docs        [][]float32
	pks         []string
	assignments []int
	inverted    [][]int
	trained     bool
	trainOnce   sync.Once
	deleted     []bool
	liveCount   int
}

func NewIVFIndex(dimension int, metricType types.MetricType, nList, nIters int) *IVFIndex {
	nprobe := int(math.Sqrt(float64(nList)))
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > nList {
		nprobe = nList
	}
	return &IVFIndex{
		dimension:  dimension,
		metricType: metricType,
		distFn:     metric.GetDistanceFunc(metricType),
		nList:      nList,
		nIters:     nIters,
		nprobe:     nprobe,
		docs:       make([][]float32, 0),
		pks:        make([]string, 0),
	}
}

func (idx *IVFIndex) SetNProbe(nprobe int) {
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > idx.nList {
		nprobe = idx.nList
	}
	idx.nprobe = nprobe
}

func (idx *IVFIndex) Add(vector []float32, pk string) uint64 {
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
	idx.deleted = append(idx.deleted, false)
	idx.liveCount++

	if idx.trained {
		nearest := idx.findNearestCentroid(v)
		idx.assignments = append(idx.assignments, nearest)
		idx.inverted[nearest] = append(idx.inverted[nearest], int(docID))
	} else {
		idx.assignments = append(idx.assignments, -1)
	}

	return docID
}

func (idx *IVFIndex) Train() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.trained {
		return
	}
	idx.trainLocked()
}

func (idx *IVFIndex) trainLocked() {
	n := len(idx.docs)
	if n == 0 {
		return
	}

	k := idx.nList
	if k > n {
		k = n
	}
	if k <= 0 {
		k = 1
	}

	idx.centroids = kmeansPP(idx.docs, k, idx.distFn, idx.nIters)
	idx.inverted = make([][]int, k)
	idx.assignments = make([]int, n)

	for i, v := range idx.docs {
		nearest := idx.findNearestCentroid(v)
		idx.assignments[i] = nearest
		idx.inverted[nearest] = append(idx.inverted[nearest], i)
	}

	idx.trained = true
}

func (idx *IVFIndex) Search(query []float32, topK int) []flat.SearchResult {
	idx.trainOnce.Do(func() {
		idx.mu.Lock()
		if !idx.trained {
			idx.trainLocked()
		}
		idx.mu.Unlock()
	})

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.docs) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	nprobe := idx.nprobe
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > idx.nList {
		nprobe = idx.nList
	}
	if nprobe > len(idx.centroids) {
		nprobe = len(idx.centroids)
	}

	centroidDists := make([]struct {
		idx  int
		dist float32
	}, len(idx.centroids))
	for i, c := range idx.centroids {
		centroidDists[i] = struct {
			idx  int
			dist float32
		}{i, idx.distFn(q, c)}
	}
	sort.Slice(centroidDists, func(i, j int) bool {
		return centroidDists[i].dist < centroidDists[j].dist
	})

	type candidate struct {
		dist  float32
		docID int
		pk    string
	}
	var candidates []candidate
	seen := make(map[int]struct{})

	for p := 0; p < nprobe; p++ {
		clusterID := centroidDists[p].idx
		for _, docIdx := range idx.inverted[clusterID] {
			if _, ok := seen[docIdx]; ok {
				continue
			}
			if idx.deleted[docIdx] {
				continue
			}
			seen[docIdx] = struct{}{}
			d := idx.distFn(q, idx.docs[docIdx])
			candidates = append(candidates, candidate{
				dist: d, docID: docIdx, pk: idx.pks[docIdx],
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]flat.SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = flat.SearchResult{
			DocID: uint64(candidates[i].docID),
			Score: 1.0 / (1.0 + candidates[i].dist),
			PK:    candidates[i].pk,
		}
	}
	return results
}

func (idx *IVFIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []flat.SearchResult {
	idx.trainOnce.Do(func() {
		idx.mu.Lock()
		if !idx.trained {
			idx.trainLocked()
		}
		idx.mu.Unlock()
	})

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.docs) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	nprobe := idx.nprobe
	if nprobe < 1 {
		nprobe = 1
	}
	if nprobe > idx.nList {
		nprobe = idx.nList
	}
	if nprobe > len(idx.centroids) {
		nprobe = len(idx.centroids)
	}

	type centroidDist struct {
		cidx int
		dist float32
	}
	cdists := make([]centroidDist, len(idx.centroids))
	for i, c := range idx.centroids {
		cdists[i] = centroidDist{cidx: i, dist: idx.distFn(q, c)}
	}
	sort.Slice(cdists, func(i, j int) bool {
		return cdists[i].dist < cdists[j].dist
	})

	type candidate struct {
		dist  float32
		docID int
		pk    string
	}
	var candidates []candidate
	seen := make(map[int]struct{})

	for p := 0; p < nprobe; p++ {
		clusterID := cdists[p].cidx
		for _, docIdx := range idx.inverted[clusterID] {
			if _, ok := seen[docIdx]; ok {
				continue
			}
			seen[docIdx] = struct{}{}
			if !filterFn(idx.pks[docIdx]) {
				continue
			}
			d := idx.distFn(q, idx.docs[docIdx])
			candidates = append(candidates, candidate{
				dist: d, docID: docIdx, pk: idx.pks[docIdx],
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	if topK > len(candidates) {
		topK = len(candidates)
	}

	results := make([]flat.SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = flat.SearchResult{
			DocID: uint64(candidates[i].docID),
			Score: 1.0 / (1.0 + candidates[i].dist),
			PK:    candidates[i].pk,
		}
	}
	return results
}

func (idx *IVFIndex) Delete(pk string) bool {
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

func (idx *IVFIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.liveCount
}

func (idx *IVFIndex) Dimension() int {
	return idx.dimension
}

func (idx *IVFIndex) Close() error {
	return nil
}

func (idx *IVFIndex) findNearestCentroid(vec []float32) int {
	bestDist := float32(math.MaxFloat32)
	bestIdx := 0
	for i, c := range idx.centroids {
		d := idx.distFn(vec, c)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

func kmeansPP(data [][]float32, k int, distFn metric.DistanceFunc, maxIters int) [][]float32 {
	n := len(data)
	if n == 0 {
		return nil
	}
	dim := len(data[0])
	rng := rand.New(rand.NewSource(42))

	centroids := make([][]float32, k)
	firstIdx := rng.Intn(n)
	centroids[0] = make([]float32, dim)
	copy(centroids[0], data[firstIdx])

	for c := 1; c < k; c++ {
		minDists := make([]float64, n)
		var totalDist float64
		for i := 0; i < n; i++ {
			minDist := math.MaxFloat64
			for j := 0; j < c; j++ {
				d := float64(distFn(data[i], centroids[j]))
				if d < minDist {
					minDist = d
				}
			}
			minDists[i] = minDist
			totalDist += minDist * minDist
		}

		r := rng.Float64() * totalDist
		cumulative := 0.0
		selected := 0
		for i, d := range minDists {
			cumulative += d * d
			if cumulative >= r {
				selected = i
				break
			}
		}

		centroids[c] = make([]float32, dim)
		copy(centroids[c], data[selected])
	}

	assignments := make([]int, n)
	for iter := 0; iter < maxIters; iter++ {
		changed := false
		for i, vec := range data {
			best := 0
			bestDist := float32(math.MaxFloat32)
			for j, c := range centroids {
				d := distFn(vec, c)
				if d < bestDist {
					bestDist = d
					best = j
				}
			}
			if assignments[i] != best {
				assignments[i] = best
				changed = true
			}
		}

		if !changed {
			break
		}

		newCentroids := make([][]float32, k)
		counts := make([]int, k)
		for i := range newCentroids {
			newCentroids[i] = make([]float32, dim)
		}

		for i, vec := range data {
			ci := assignments[i]
			for j, v := range vec {
				newCentroids[ci][j] += v
			}
			counts[ci]++
		}

		for i := 0; i < k; i++ {
			if counts[i] > 0 {
				for j := range newCentroids[i] {
					newCentroids[i][j] /= float32(counts[i])
				}
			} else {
				copy(newCentroids[i], centroids[i])
			}
		}

		centroids = newCentroids
	}

	return centroids
}
