package vamana

import (
	"bufio"
	"container/heap"
	"fmt"
	"io"
	"log"
	"math/rand"
	"runtime"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/persist"
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
	pkToDocID  map[string]int
	graph      [][]int
	rng        *rand.Rand
	entryPoint int
	deleted    []bool
	liveCount  int

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
		pkToDocID:      make(map[string]int),
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
	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	idx.mu.Lock()

	docID := uint64(len(idx.docs))
	idx.docs = append(idx.docs, v)
	idx.pks = append(idx.pks, pk)
	idx.pkToDocID[pk] = len(idx.pks) - 1
	idx.graph = append(idx.graph, []int{})
	idx.deleted = append(idx.deleted, false)
	idx.liveCount++

	if idx.entryPoint < 0 {
		idx.entryPoint = 0
		idx.mu.Unlock()
		return docID
	}

	entryPoint := idx.entryPoint
	searchListSize := idx.searchListSize
	saturateGraph := idx.saturateGraph
	idx.mu.Unlock()

	idx.mu.RLock()
	candidates := idx.greedySearch(v, entryPoint, searchListSize)
	idx.mu.RUnlock()

	idx.mu.Lock()
	idx.pruneAndAdd(int(docID), candidates)
	if saturateGraph {
		idx.ensureDegreeFromCandidates(int(docID), candidates)
	}
	idx.mu.Unlock()

	return docID
}

func (idx *VamanaIndex) BatchBuild(vectors [][]float32, pks []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	n := len(vectors)
	if n == 0 {
		return
	}

	idx.docs = make([][]float32, n)
	idx.pks = make([]string, n)
	idx.graph = make([][]int, n)
	idx.deleted = make([]bool, n)
	idx.liveCount = n

	for i, v := range vectors {
		vc := make([]float32, len(v))
		copy(vc, v)
		if idx.metricType == types.MetricTypeCosine {
			metric.NormalizeInPlace(vc)
		}
		idx.docs[i] = vc
		if i < len(pks) {
			idx.pks[i] = pks[i]
		} else {
			idx.pks[i] = fmt.Sprintf("d%d", i)
		}
	}

	idx.entryPoint = 0

	for i := range idx.graph {
		neighbors := idx.nearestNeighborInit(i, idx.maxDegree)
		idx.graph[i] = neighbors
	}

	graphSnapshot := make([][]int, n)
	for i := range idx.graph {
		graphSnapshot[i] = make([]int, len(idx.graph[i]))
		copy(graphSnapshot[i], idx.graph[i])
	}

	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	if numWorkers > 16 {
		numWorkers = 16
	}
	chunkSize := (n + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup

	localGraphs := make([][][]int, numWorkers)

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= n {
			break
		}
		wg.Add(1)
		go func(workerID, start, end int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in concurrentBuild worker: %v", r)
				}
			}()
			local := make([][]int, end-start)
			for i := start; i < end; i++ {
				candidates := greedySearchOnGraph(graphSnapshot, idx.docs, idx.distFn, idx.docs[i], idx.entryPoint, idx.searchListSize)
				pruned := idx.robustPrune(i, candidates)
				local[i-start] = pruned
			}
			localGraphs[workerID] = local
		}(w, start, end)
	}
	wg.Wait()

	for w := 0; w < numWorkers; w++ {
		if localGraphs[w] == nil {
			continue
		}
		start := w * chunkSize
		for i, neighbors := range localGraphs[w] {
			idx.graph[start+i] = neighbors
		}
	}

	for i := range idx.graph {
		for _, nb := range idx.graph[i] {
			found := false
			for _, x := range idx.graph[nb] {
				if x == i {
					found = true
					break
				}
			}
			if !found {
				idx.graph[nb] = append(idx.graph[nb], i)
			}
		}
	}

	for i := range idx.graph {
		if len(idx.graph[i]) > idx.maxDegree*2 {
			neighbors := make([]vNeighbor, len(idx.graph[i]))
			for j, nb := range idx.graph[i] {
				neighbors[j] = vNeighbor{id: nb, dist: idx.distFn(idx.docs[i], idx.docs[nb])}
			}
			sort.Slice(neighbors, func(a, b int) bool {
				return neighbors[a].dist < neighbors[b].dist
			})
			if len(neighbors) > idx.maxDegree*2 {
				neighbors = neighbors[:idx.maxDegree*2]
			}
			trimmed := make([]int, len(neighbors))
			for j, nn := range neighbors {
				trimmed[j] = nn.id
			}
			idx.graph[i] = trimmed
		}
	}
}

func greedySearchOnGraph(graph [][]int, docs [][]float32, distFn metric.DistanceFunc, query []float32, start int, L int) []vNeighbor {
	n := len(docs)
	visited := make([]byte, n)
	visited[start] = 1

	candidates := &vMinHeap{vNeighbor{id: start, dist: distFn(query, docs[start])}}
	heap.Init(candidates)

	result := &vMaxHeap{vNeighbor{id: start, dist: distFn(query, docs[start])}}
	heap.Init(result)

	for candidates.Len() > 0 {
		closest := (*candidates)[0]

		if result.Len() >= L && closest.dist > (*result)[0].dist {
			break
		}

		heap.Pop(candidates)

		for _, nb := range graph[closest.id] {
			if visited[nb] != 0 {
				continue
			}
			visited[nb] = 1
			dist := distFn(query, docs[nb])
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

func (idx *VamanaIndex) cleanupGraphForDeletedNode(nodeID int) {
	for i, nbs := range idx.graph {
		if i == nodeID {
			continue
		}
		filtered := make([]int, 0, len(nbs))
		for _, nb := range nbs {
			if nb != nodeID {
				filtered = append(filtered, nb)
			}
		}
		if len(filtered) != len(nbs) {
			idx.graph[i] = filtered
		}
	}
}

func (idx *VamanaIndex) nearestNeighborInit(nodeID int, k int) []int {
	n := len(idx.docs)
	if n <= 1 {
		return nil
	}
	if k >= n-1 {
		neighbors := make([]int, 0, n-1)
		for j := 0; j < n; j++ {
			if j != nodeID {
				neighbors = append(neighbors, j)
			}
		}
		return neighbors
	}

	type distEntry struct {
		id   int
		dist float32
	}
	entries := make([]distEntry, 0, n-1)
	for j := 0; j < n; j++ {
		if j == nodeID {
			continue
		}
		entries = append(entries, distEntry{id: j, dist: idx.distFn(idx.docs[nodeID], idx.docs[j])})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].dist < entries[j].dist
	})

	result := make([]int, 0, k)
	for i := 0; i < k && i < len(entries); i++ {
		result = append(result, entries[i].id)
	}
	return result
}

func (idx *VamanaIndex) robustPrune(nodeID int, candidates []vNeighbor) []int {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	var pruned []int
	for _, c := range candidates {
		if c.id == nodeID {
			continue
		}
		keep := true
		for _, p := range pruned {
			distPC := idx.distFn(idx.docs[p], idx.docs[c.id])
			if idx.alpha*distPC <= c.dist {
				keep = false
				break
			}
		}
		if keep {
			pruned = append(pruned, c.id)
			if len(pruned) >= idx.maxDegree {
				break
			}
		}
	}
	return pruned
}

func (idx *VamanaIndex) Search(query []float32, topK int) []types.SearchResult {
	return idx.SearchWithListSize(query, topK, 0)
}

func (idx *VamanaIndex) SearchWithListSize(query []float32, topK, searchListSize int) []types.SearchResult {
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

	results := make([]types.SearchResult, 0, topK)
	for i := 0; i < len(candidates) && len(results) < topK; i++ {
		c := candidates[i]
		if idx.deleted[c.id] {
			continue
		}
		results = append(results, types.SearchResult{
			DocID: uint64(c.id),
			Score: 1.0 / (1.0 + c.dist),
			PK:    idx.pks[c.id],
		})
	}
	return results
}

func (idx *VamanaIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []types.SearchResult {
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

	maxL := len(idx.docs)
	L := topK * 2
	if L < idx.searchListSize {
		L = idx.searchListSize
	}
	if L > maxL {
		L = maxL
	}

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

	visited[idx.entryPoint] = 1
	startDist := idx.distFn(q, idx.docs[idx.entryPoint])
	startN := vNeighbor{id: idx.entryPoint, dist: startDist}

	candidates := &vMinHeap{startN}
	heap.Init(candidates)
	resultHeap := &vMaxHeap{startN}
	heap.Init(resultHeap)

	var results []types.SearchResult
	lastL := 0
	for L <= maxL && L != lastL {
		lastL = L

		idx.expandGreedySearch(q, L, visited, candidates, resultHeap)

		results = nil
		sorted := make([]vNeighbor, resultHeap.Len())
		for i, nb := range *resultHeap {
			sorted[i] = nb
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].dist < sorted[j].dist
		})

		for _, c := range sorted {
			if idx.deleted[c.id] {
				continue
			}
			if filterFn(idx.pks[c.id]) {
				results = append(results, types.SearchResult{
					DocID: uint64(c.id),
					Score: 1.0 / (1.0 + c.dist),
					PK:    idx.pks[c.id],
				})
				if len(results) >= topK {
					*visitedPtr = visited[:cap(visited)]
					idx.visitedPool.Put(visitedPtr)
					return results
				}
			}
		}

		if len(results) >= topK {
			break
		}

		L *= 2
		if L > maxL {
			L = maxL
		}
	}

	*visitedPtr = visited[:cap(visited)]
	idx.visitedPool.Put(visitedPtr)
	return results
}

func (idx *VamanaIndex) expandGreedySearch(query []float32, L int,
	visited []byte, candidates *vMinHeap, result *vMaxHeap) {

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

			if idx.deleted[nb] {
				for _, nnb := range idx.graph[nb] {
					if visited[nnb] != 0 {
						continue
					}
					visited[nnb] = 1
					if idx.deleted[nnb] {
						continue
					}
					dist := idx.distFn(query, idx.docs[nnb])
					vn := vNeighbor{id: nnb, dist: dist}
					if result.Len() < L || dist < (*result)[0].dist {
						heap.Push(candidates, vn)
						heap.Push(result, vn)
						if result.Len() > L {
							heap.Pop(result)
						}
					}
				}
				continue
			}

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
}

func (idx *VamanaIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	i, ok := idx.pkToDocID[pk]
	if !ok || idx.deleted[i] {
		return false
	}
	idx.deleted[i] = true
	idx.liveCount--
	delete(idx.pkToDocID, pk)

	idx.cleanupGraphForDeletedNode(i)

	if i == idx.entryPoint {
		newEP := -1
		for _, nb := range idx.graph[i] {
			if !idx.deleted[nb] {
				newEP = nb
				break
			}
		}
		if newEP < 0 {
			for j := range idx.docs {
				if !idx.deleted[j] {
					newEP = j
					break
				}
			}
		}
		idx.entryPoint = newEP
	}

	return true
}

func (idx *VamanaIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.liveCount
}

func (idx *VamanaIndex) Dimension() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.dimension
}

func (idx *VamanaIndex) MemoryBytes() uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var total uint64
	for _, v := range idx.docs {
		total += uint64(len(v)) * 4
	}
	for _, pk := range idx.pks {
		total += uint64(len(pk))
	}
	total += uint64(len(idx.deleted))
	for _, nbs := range idx.graph {
		total += uint64(len(nbs)) * 4
	}
	return total
}

func (idx *VamanaIndex) Save(w io.Writer) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if err := persist.WriteHeader(bw, persist.FileHeader{Magic: persist.MagicNumber, Version: 1, IndexType: persist.IndexTypeVamana}); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.dimension); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, int(idx.metricType)); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.maxDegree); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.searchListSize); err != nil {
		return err
	}
	if err := persist.WriteFloat32(bw, idx.alpha); err != nil {
		return err
	}
	if err := persist.WriteByte(bw, boolToByte(idx.saturateGraph)); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.entryPoint); err != nil {
		return err
	}
	if err := persist.WriteInt(bw, idx.liveCount); err != nil {
		return err
	}

	if err := persist.WriteUint32(bw, uint32(len(idx.docs))); err != nil {
		return err
	}
	for _, v := range idx.docs {
		if err := persist.WriteFloat32Slice(bw, v); err != nil {
			return err
		}
	}

	if err := persist.WriteUint32(bw, uint32(len(idx.pks))); err != nil {
		return err
	}
	for _, pk := range idx.pks {
		if err := persist.WriteString(bw, pk); err != nil {
			return err
		}
	}

	if err := persist.WriteUint32(bw, uint32(len(idx.graph))); err != nil {
		return err
	}
	for _, nbs := range idx.graph {
		if err := persist.WriteIntSlice(bw, nbs); err != nil {
			return err
		}
	}

	return persist.WriteBoolSlice(bw, idx.deleted)
}

func (idx *VamanaIndex) Load(r io.Reader) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	br := bufio.NewReader(r)

	h, err := persist.ReadHeader(br)
	if err != nil {
		return err
	}
	if h.IndexType != persist.IndexTypeVamana {
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
	maxDegree, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	searchListSize, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	alpha, err := persist.ReadFloat32(br)
	if err != nil {
		return err
	}
	satBuf := make([]byte, 1)
	if _, err := br.Read(satBuf); err != nil {
		return err
	}
	satByte := satBuf[0]
	entryPoint, err := persist.ReadInt(br)
	if err != nil {
		return err
	}
	liveCount, err := persist.ReadInt(br)
	if err != nil {
		return err
	}

	docCount, err := persist.ReadUint32(br)
	if err != nil {
		return err
	}
	docs := make([][]float32, docCount)
	for i := range docs {
		docs[i], err = persist.ReadFloat32Slice(br)
		if err != nil {
			return err
		}
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

	graphCount, err := persist.ReadUint32(br)
	if err != nil {
		return err
	}
	graph := make([][]int, graphCount)
	for i := range graph {
		graph[i], err = persist.ReadIntSlice(br)
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
	idx.maxDegree = maxDegree
	idx.searchListSize = searchListSize
	idx.alpha = alpha
	idx.saturateGraph = satByte != 0
	idx.entryPoint = entryPoint
	idx.liveCount = liveCount
	idx.docs = docs
	idx.pks = pks
	idx.graph = graph
	idx.deleted = deleted
	return nil
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
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

			if idx.deleted[nb] {
				for _, nnb := range idx.graph[nb] {
					if visited[nnb] != 0 {
						continue
					}
					visited[nnb] = 1
					if idx.deleted[nnb] {
						continue
					}
					dist := idx.distFn(query, idx.docs[nnb])
					vn := vNeighbor{id: nnb, dist: dist}
					if result.Len() < L || dist < (*result)[0].dist {
						heap.Push(candidates, vn)
						heap.Push(result, vn)
						if result.Len() > L {
							heap.Pop(result)
						}
					}
				}
				continue
			}

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
	neighbors := make([]vNeighbor, 0, len(nbs))
	for _, nb := range nbs {
		if idx.deleted[nb] {
			continue
		}
		neighbors = append(neighbors, vNeighbor{id: nb, dist: idx.distFn(idx.docs[nodeID], idx.docs[nb])})
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
