package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
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
	numDocs   = 50000
	topK      = 10
	numQuery  = 100
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
	resultCount int
}

func printResult(r benchResult) {
	fmt.Printf("\n[%s]\n", r.indexType)
	fmt.Printf("  插入耗时: %d ms (%.0f docs/s)\n", r.insertMs, float64(numDocs)/float64(r.insertMs)*1000)
	fmt.Printf("  搜索耗时: %d ms (%d queries)\n", r.searchMs, numQuery)
	fmt.Printf("  QPS: %.0f | 平均延迟: %.3f ms | 结果数: %d\n", r.qps, r.avgLatency, r.resultCount)
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

func runSearchBench(c *collection.Collection, queryVecs [][]float32) (int64, float64, float64, int) {
	start := time.Now()
	count := 0
	for i := 0; i < numQuery; i++ {
		results, st := c.Query(&query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: queryVecs[i]},
			},
			TopK: topK,
		})
		if !st.OK() {
			log.Fatalf("查询失败: %v", st.Error())
		}
		count = len(results)
	}
	elapsed := time.Since(start)
	totalMs := elapsed.Milliseconds()
	qps := float64(numQuery) / elapsed.Seconds()
	avgLatency := float64(totalMs) / float64(numQuery)
	return totalMs, qps, avgLatency, count
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
	searchMs, qps, avgLat, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"Flat", insertMs, searchMs, qps, avgLat, cnt})
}

func benchHNSW(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200)}
	c, dir := createBenchCollection("hnsw", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"HNSW (M=16, efConstruction=200)", insertMs, searchMs, qps, avgLat, cnt})
}

func benchIVF(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewIVFIndexParams(types.MetricTypeCosine, 16, 20, false)}
	c, dir := createBenchCollection("ivf", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"IVF (nList=16, nIters=20)", insertMs, searchMs, qps, avgLat, cnt})
}

func benchVamana(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false)}
	c, dir := createBenchCollection("vamana", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"Vamana (maxDegree=16, alpha=1.2)", insertMs, searchMs, qps, avgLat, cnt})
}

func benchHNSWRabitq(vectors, queryVecs [][]float32) {
	p := &IndexParams{params: param.NewHNSWRabitqIndexParams(types.MetricTypeCosine, 1, 256, 16, 200, 1000)}
	c, dir := createBenchCollection("hnsw_rabitq", p)
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertBenchData(c, vectors)
	searchMs, qps, avgLat, cnt := runSearchBench(c, queryVecs)
	printResult(benchResult{"HNSW RaBitQ (M=16, totalBits=1)", insertMs, searchMs, qps, avgLat, cnt})
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
	fmt.Printf("\n[Concurrent Vamana (BatchQuery, %d goroutines)]\n", len(queries))
	fmt.Printf("  搜索耗时: %d ms (%d queries)\n", elapsed.Milliseconds(), len(queries))
	fmt.Printf("  QPS: %.0f | 平均延迟: %.3f ms | 结果数: %d\n", qps, avgLat, len(results[0]))
}