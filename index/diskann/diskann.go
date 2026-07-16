package diskann

import (
	"container/list"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/storage"
	"github.com/third-apps/go-zvec/types"
)

type candidate struct {
	id   uint64
	dist float32
}

type cacheEntry struct {
	key uint64
	vec []float32
}

type lruCache struct {
	mu       sync.Mutex
	capacity int
	items    map[uint64]*list.Element
	order    *list.List
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[uint64]*list.Element),
		order:    list.New(),
	}
}

func (c *lruCache) Get(key uint64) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).vec, true
	}
	return nil, false
}

func (c *lruCache) Put(key uint64, vec []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).vec = vec
		return
	}
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			entry := oldest.Value.(*cacheEntry)
			delete(c.items, entry.key)
		}
	}
	entry := &cacheEntry{key: key, vec: vec}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

func (c *lruCache) Delete(key uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.Remove(elem)
		delete(c.items, key)
	}
}

type DiskAnnIndex struct {
	mu sync.RWMutex

	dimension  int
	metricType types.MetricType
	distFn     metric.DistanceFunc

	maxDegree     int
	searchList    int
	alpha         float64
	saturateGraph bool

	adjList    [][]uint64
	pks        []string
	enterPoint int
	rng        *rand.Rand

	vectorStore storage.Storage
	vectorCache *lruCache
	cacheMax    int

	path      string
	persisted bool
}

func NewDiskAnnIndex(dimension int, metricType types.MetricType,
	maxDegree, searchList int, alpha float64, saturateGraph bool) *DiskAnnIndex {

	return &DiskAnnIndex{
		dimension:     dimension,
		metricType:    metricType,
		distFn:        metric.GetDistanceFunc(metricType),
		maxDegree:     maxDegree,
		searchList:    searchList,
		alpha:         alpha,
		saturateGraph: saturateGraph,
		adjList:       make([][]uint64, 0),
		pks:           make([]string, 0),
		enterPoint:    -1,
		rng:         rand.New(rand.NewSource(42)),
		vectorCache: newLRUCache(4096),
		cacheMax:    4096,
	}
}

func (idx *DiskAnnIndex) SetPath(path string) {
	idx.path = path
}

func (idx *DiskAnnIndex) InitStorage() error {
	if idx.path == "" {
		return nil
	}
	storePath := filepath.Join(idx.path, "vectors.bin")
	store, err := storage.NewMMAPStorage(storePath, int64(idx.dimension*4*1024), storage.StorageOptions{CreateNew: true})
	if err != nil {
		store2, err2 := storage.OpenFileStorage(storePath, storage.StorageOptions{CreateNew: true})
		if err2 != nil {
			return err2
		}
		idx.vectorStore = store2
	} else {
		idx.vectorStore = store
	}
	return nil
}

func (idx *DiskAnnIndex) cacheGet(docID uint64) ([]float32, bool) {
	v, ok := idx.vectorCache.Get(docID)
	return v, ok
}

func (idx *DiskAnnIndex) cachePut(docID uint64, vec []float32) {
	idx.vectorCache.Put(docID, vec)
}

func (idx *DiskAnnIndex) getVector(docID uint64) []float32 {
	if v, ok := idx.cacheGet(docID); ok {
		return v
	}
	if idx.vectorStore != nil {
		buf := make([]byte, idx.dimension*4)
		_, err := idx.vectorStore.Read(int64(docID)*int64(idx.dimension*4), buf)
		if err == nil {
			vec := make([]float32, idx.dimension)
			for j := 0; j < idx.dimension; j++ {
				vec[j] = math.Float32frombits(
					uint32(buf[j*4]) | uint32(buf[j*4+1])<<8 |
						uint32(buf[j*4+2])<<16 | uint32(buf[j*4+3])<<24)
			}
			idx.cachePut(docID, vec)
			return vec
		}
	}
	return nil
}

func (idx *DiskAnnIndex) putVector(docID uint64, vec []float32) {
	idx.cachePut(docID, vec)
	if idx.vectorStore != nil {
		buf := make([]byte, idx.dimension*4)
		for j := 0; j < idx.dimension; j++ {
			bits := math.Float32bits(vec[j])
			buf[j*4] = byte(bits)
			buf[j*4+1] = byte(bits >> 8)
			buf[j*4+2] = byte(bits >> 16)
			buf[j*4+3] = byte(bits >> 24)
		}
		idx.vectorStore.Write(int64(docID)*int64(idx.dimension*4), buf)
	}
}

func (idx *DiskAnnIndex) Add(vector []float32, pk string) uint64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	v := make([]float32, len(vector))
	copy(v, vector)
	if idx.metricType == types.MetricTypeCosine {
		v = metric.Normalize(v)
	}

	docID := uint64(len(idx.pks))
	idx.pks = append(idx.pks, pk)
	idx.adjList = append(idx.adjList, []uint64{})
	idx.putVector(docID, v)

	if idx.enterPoint < 0 {
		idx.enterPoint = int(docID)
		return docID
	}

	idx.pruneAndAdd(docID, v)
	return docID
}

func (idx *DiskAnnIndex) Search(query []float32, topK int) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.searchLocked(query, topK)
}

func (idx *DiskAnnIndex) searchLocked(query []float32, topK int) []flat.SearchResult {
	if idx.enterPoint < 0 || len(idx.pks) == 0 {
		return nil
	}

	q := make([]float32, len(query))
	copy(q, query)
	if idx.metricType == types.MetricTypeCosine {
		q = metric.Normalize(q)
	}

	visited := make(map[uint64]struct{})
	results := idx.greedySearch(q, uint64(idx.enterPoint), idx.searchList, visited)

	sort.Slice(results, func(i, j int) bool {
		return results[i].dist < results[j].dist
	})

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]flat.SearchResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = flat.SearchResult{
			DocID: results[i].id,
			Score: 1.0 / (1.0 + results[i].dist),
			PK:    idx.pks[results[i].id],
		}
	}
	return out
}

func (idx *DiskAnnIndex) SearchWithFilter(query []float32, topK int,
	filterFn func(pk string) bool) []flat.SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	all := idx.searchLocked(query, len(idx.pks))
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

func (idx *DiskAnnIndex) Delete(pk string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.pks {
		if p == pk {
			idx.vectorCache.Delete(uint64(i))
			idx.pks = append(idx.pks[:i], idx.pks[i+1:]...)
			idx.adjList = append(idx.adjList[:i], idx.adjList[i+1:]...)
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
				if len(idx.pks) > 0 {
					idx.enterPoint = 0
				} else {
					idx.enterPoint = -1
				}
			} else if idx.enterPoint > i {
				idx.enterPoint--
			}
			return true
		}
	}
	return false
}

func (idx *DiskAnnIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.pks)
}

func (idx *DiskAnnIndex) Dimension() int {
	return idx.dimension
}

func (idx *DiskAnnIndex) greedySearch(query []float32, start uint64, k int, visited map[uint64]struct{}) []candidate {
	var cand []candidate
	var results []candidate

	vec := idx.getVector(start)
	if vec == nil {
		return nil
	}
	startDist := idx.distFn(query, vec)
	startCand := candidate{id: start, dist: startDist}

	cand = append(cand, startCand)
	results = append(results, startCand)
	visited[start] = struct{}{}

	for len(cand) > 0 {
		closest := cand[0]
		cand = cand[1:]

		farthestDist := float32(math.MaxFloat32)
		if len(results) >= k {
			farthestDist = results[len(results)-1].dist
		}

		if closest.dist > farthestDist {
			break
		}

		for _, nb := range idx.adjList[closest.id] {
			if _, seen := visited[nb]; seen {
				continue
			}
			visited[nb] = struct{}{}

			nbVec := idx.getVector(nb)
			if nbVec == nil {
				continue
			}
			nbDist := idx.distFn(query, nbVec)

			if len(results) < k || nbDist < farthestDist {
				nc := candidate{id: nb, dist: nbDist}
				cand = append(cand, nc)
				results = append(results, nc)
				sort.Slice(results, func(i, j int) bool {
					return results[i].dist < results[j].dist
				})
				if len(results) > k {
					results = results[:k]
				}
				if len(results) >= k {
					farthestDist = results[len(results)-1].dist
				}
			}
		}
	}

	return results
}

func (idx *DiskAnnIndex) pruneAndAdd(newID uint64, newVec []float32) {
	visited := make(map[uint64]struct{})
	candidates := idx.greedySearch(newVec, uint64(idx.enterPoint), idx.searchList, visited)

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	maxCandidates := idx.maxDegree
	if idx.saturateGraph {
		maxCandidates = idx.searchList
	}
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	connected := make(map[uint64]bool)
	for _, c := range candidates {
		connected[c.id] = true
	}

	for _, c := range candidates {
		if len(idx.adjList[newID]) >= idx.maxDegree {
			break
		}
		idx.addUndirectedEdge(newID, c.id)
	}

	for _, c := range candidates {
		idx.trimNeighbors(c.id, newID)
	}

	if idx.saturateGraph {
		for i := 0; i < len(idx.pks)-1; i++ {
			if uint64(i) == newID {
				continue
			}
			idx.trimNeighbors(uint64(i), newID)
		}
	}
}

func (idx *DiskAnnIndex) addUndirectedEdge(a, b uint64) {
	idx.adjList[a] = append(idx.adjList[a], b)
	idx.adjList[b] = append(idx.adjList[b], a)
}

func (idx *DiskAnnIndex) trimNeighbors(nodeID uint64, newID uint64) {
	nbs := idx.adjList[nodeID]
	if len(nbs) <= idx.maxDegree {
		return
	}

	nodeVec := idx.getVector(nodeID)
	if nodeVec == nil {
		return
	}

	type nbDist struct {
		id   uint64
		dist float32
	}
	dists := make([]nbDist, 0, len(nbs))
	for _, nb := range nbs {
		nbVec := idx.getVector(nb)
		if nbVec == nil {
			continue
		}
		d := idx.distFn(nodeVec, nbVec)
		dists = append(dists, nbDist{id: nb, dist: d})
	}

	sort.Slice(dists, func(i, j int) bool {
		return dists[i].dist < dists[j].dist
	})

	if len(dists) == 0 {
		idx.adjList[nodeID] = nil
		return
	}

	kept := []uint64{dists[0].id}
	for i := 1; i < len(dists); i++ {
		prune := false
		for _, k := range kept {
			kVec := idx.getVector(k)
			dVec := idx.getVector(dists[i].id)
			if kVec == nil || dVec == nil {
				continue
			}
			if idx.distFn(kVec, dVec)*float32(idx.alpha) <= dists[i].dist {
				prune = true
				break
			}
		}
		if !prune {
			kept = append(kept, dists[i].id)
		}
		if len(kept) >= idx.maxDegree {
			break
		}
	}
	idx.adjList[nodeID] = kept
}

func (idx *DiskAnnIndex) Sync() error {
	if idx.vectorStore != nil {
		return idx.vectorStore.Sync()
	}
	return nil
}

func (idx *DiskAnnIndex) Close() error {
	if idx.vectorStore != nil {
		return idx.vectorStore.Close()
	}
	return nil
}
