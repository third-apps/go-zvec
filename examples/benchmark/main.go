package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	zvec "github.com/third-apps/go-zvec"
	"github.com/third-apps/go-zvec/collection"
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

const (
	dim       = 128
	numDocs   = 100000
	topK      = 10
	numQuery  = 10000
	batchSize = 5000
)

func main() {
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("============================================")
	fmt.Println("  Go-Zvec 性能基准测试 / Performance Benchmark")
	fmt.Println("============================================")
	fmt.Printf("向量维度: %d | 文档数: %d | TopK: %d | 查询次数: %d\n", dim, numDocs, topK, numQuery)
	fmt.Printf("Go版本: %s | CPU核心: %d | OS: %s\n", runtime.Version(), runtime.NumCPU(), runtime.GOOS)
	fmt.Println("============================================")

	vectors := generateVectors(numDocs, dim)
	queryVecs := generateVectors(numQuery, dim)

	benchFlat(vectors, queryVecs)
	benchIVF(vectors, queryVecs)
	benchHNSW(vectors, queryVecs)
	benchVamana(vectors, queryVecs)
	benchHNSWRabitq(vectors, queryVecs)
	benchConcurrentSearch(vectors, queryVecs)
	benchRealConcurrentSearch(vectors, queryVecs)

	fmt.Println("\n============================================")
	fmt.Println("  基准测试完成 / Benchmark Complete")
	fmt.Println("============================================")
}

type benchResult struct {
	indexType   string
	insertMs    int64
	searchMs    int64
	qps         float64
	avgLatency  float64
	p50         float64
	p95         float64
	p99         float64
	resultCount int
}

func printResult(r benchResult, memBytes uint64) {
	fmt.Printf("\n[%s]\n", r.indexType)
	fmt.Printf("  插入耗时: %d ms (%.0f docs/s)\n", r.insertMs, float64(numDocs)/float64(r.insertMs)*1000)
	fmt.Printf("  搜索耗时: %d ms (%d queries)\n", r.searchMs, numQuery)
	fmt.Printf("  QPS: %.0f | 平均延迟: %.3f ms\n", r.qps, r.avgLatency)
	fmt.Printf("  P50: %.3f ms | P95: %.3f ms | P99: %.3f ms | 结果数: %d\n", r.p50, r.p95, r.p99, r.resultCount)
	fmt.Printf("  内存占用: %.2f MB\n", float64(memBytes)/1024/1024)
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100.0 * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func generateVectors(n, d int) [][]float32 {
	rng := rand.New(rand.NewSource(42))
	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		v := make([]float32, d)
		var sum float32
		for j := 0; j < d; j++ {
			v[j] = rng.Float32()
			sum += v[j] * v[j]
		}
		norm := float32(1.0 / float64(sum))
		for j := 0; j < d; j++ {
			v[j] *= norm
		}
		vecs[i] = v
	}
	return vecs
}

func createBenchCollection(name string, indexParams *IndexParams) (*collection.Collection, string) {
	dir := filepath.Join(os.TempDir(), "go-zvec-bench-"+name)
	os.RemoveAll(dir)

	s := schema.NewCollectionSchema(name)
	f := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, dim)
	f.SetIndexParams(indexParams.params)
	s.AddField(f)

	c, err := zvec.CreateAndOpen(dir, s, &collection.Options{})
	if err != nil {
		log.Fatalf("[%s] 创建集合失败: %v", name, err)
	}
	return c, dir
}

func insertBenchData(c *collection.Collection, vectors [][]float32) int64 {
	start := time.Now()
	for i := 0; i < len(vectors); i += batchSize {
		end := i + batchSize
		if end > len(vectors) {
			end = len(vectors)
		}
		docs := make([]*doc.Doc, end-i)
		for j := i; j < end; j++ {
			d := doc.NewDoc(fmt.Sprintf("d%d", j))
			d.SetVectorFP32Field("vec", vectors[j])
			docs[j-i] = d
		}
		if st := c.Insert(docs); !st.OK() {
			log.Fatalf("插入失败: %v", st.Error())
		}
	}
	return time.Since(start).Milliseconds()
}

func runSearchBench(c *collection.Collection, queryVecs [][]float32) (int64, float64, float64, float64, float64, float64, int) {
	latencies := make([]float64, numQuery)
	count := 0
	start := time.Now()
	for i := 0; i < numQuery; i++ {
		qStart := time.Now()
		results, st := c.Query(&query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: queryVecs[i]},
			},
			TopK: topK,
		})
		latencies[i] = float64(time.Since(qStart).Nanoseconds()) / 1e6
		if !st.OK() {
			log.Fatalf("查询失败: %v", st.Error())
		}
		count = len(results)
	}
	elapsed := time.Since(start)
	totalMs := elapsed.Milliseconds()
	qps := float64(numQuery) / elapsed.Seconds()
	avgLatency := float64(totalMs) / float64(numQuery)

	sorted := make([]float64, numQuery)
	copy(sorted, latencies)
	sort.Float64s(sorted)
	p50 := percentile(sorted, 50)
	p95 := percentile(sorted, 95)
	p99 := percentile(sorted, 99)

	return totalMs, qps, avgLatency, p50, p95, p99, count
}

type IndexParams struct {
	params *param.IndexParams
}

func benchFlat(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewFlatIndexParams(types.MetricTypeCosine)}
	c, dir := createBenchCollection("flat", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, p50, p95, p99, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"Flat", insertMs, searchMs, qps, avgLat, p50, p95, p99, cnt}, c.Stats().TotalMemoryBytes)
}

func benchHNSW(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200)}
	c, dir := createBenchCollection("hnsw", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, p50, p95, p99, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"HNSW (M=16, efConstruction=200)", insertMs, searchMs, qps, avgLat, p50, p95, p99, cnt}, c.Stats().TotalMemoryBytes)
}

func benchIVF(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewIVFIndexParams(types.MetricTypeCosine, 128, 20, false)}
	c, dir := createBenchCollection("ivf", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, p50, p95, p99, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"IVF (nList=128, nIters=20)", insertMs, searchMs, qps, avgLat, p50, p95, p99, cnt}, c.Stats().TotalMemoryBytes)
}

func benchVamana(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false)}
	c, dir := createBenchCollection("vamana", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, p50, p95, p99, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"Vamana (maxDegree=16, alpha=1.2)", insertMs, searchMs, qps, avgLat, p50, p95, p99, cnt}, c.Stats().TotalMemoryBytes)
}

func benchHNSWRabitq(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewHNSWRabitqIndexParams(types.MetricTypeCosine, 1, 256, 16, 200, 1000)}
	c, dir := createBenchCollection("hnsw_rabitq", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, p50, p95, p99, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"HNSW RaBitQ (M=16, totalBits=1)", insertMs, searchMs, qps, avgLat, p50, p95, p99, cnt}, c.Stats().TotalMemoryBytes)
}

func benchConcurrentSearch(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false)}
	c, dir := createBenchCollection("concurrent_vamana", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertBenchData(c, vectors)

	queries := make([]*query.SearchQuery, len(queryVecs))
	for i, qv := range queryVecs {
		queries[i] = &query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: qv},
			},
			TopK: topK,
		}
	}

	start := time.Now()
	results := c.BatchQuery(queries)
	elapsed := time.Since(start)
	qps := float64(len(queries)) / elapsed.Seconds()
	avgLat := float64(elapsed.Milliseconds()) / float64(len(queries))

	stats := c.Stats()
	fmt.Printf("\n[Concurrent Vamana (BatchQuery, %d goroutines)]\n", len(queries))
	fmt.Printf("  搜索耗时: %d ms (%d queries)\n", elapsed.Milliseconds(), len(queries))
	fmt.Printf("  QPS: %.0f | 平均延迟: %.3f ms | 结果数: %d\n", qps, avgLat, len(results[0]))
	fmt.Printf("  内存占用: %.2f MB\n", float64(stats.TotalMemoryBytes)/1024/1024)
}

func benchRealConcurrentSearch(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false)}
	c, dir := createBenchCollection("real_concurrent_vamana", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertBenchData(c, vectors)

	concurrencies := []int{1, 4, 8, 16, 32}
	queryCount := 1000

	for _, conc := range concurrencies {
		latencies := make([]float64, queryCount*conc)
		var wg sync.WaitGroup
		var mu sync.Mutex
		idx := 0

		start := time.Now()
		for g := 0; g < conc; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := 0; i < queryCount; i++ {
					qi := (goroutineID*queryCount + i) % len(queryVecs)
					qStart := time.Now()
					c.Query(&query.SearchQuery{
						Target: query.QueryTarget{
							FieldName: "vec",
							Vector:    &query.VectorClause{QueryVector: queryVecs[qi]},
						},
						TopK: topK,
					})
					lat := float64(time.Since(qStart).Nanoseconds()) / 1e6
					mu.Lock()
					latencies[idx] = lat
					idx++
					mu.Unlock()
				}
			}(g)
		}
		wg.Wait()
		elapsed := time.Since(start)

		totalQueries := conc * queryCount
		qps := float64(totalQueries) / elapsed.Seconds()

		sorted := make([]float64, idx)
		copy(sorted, latencies[:idx])
		sort.Float64s(sorted)
		p50 := percentile(sorted, 50)
		p95 := percentile(sorted, 95)
		p99 := percentile(sorted, 99)
		avgLat := float64(elapsed.Milliseconds()) / float64(totalQueries)

		fmt.Printf("\n[Real Concurrent Vamana (%d goroutines × %d queries)]\n", conc, queryCount)
		fmt.Printf("  总耗时: %d ms | QPS: %.0f | 平均延迟: %.3f ms\n", elapsed.Milliseconds(), qps, avgLat)
		fmt.Printf("  P50: %.3f ms | P95: %.3f ms | P99: %.3f ms\n", p50, p95, p99)
	}
}
