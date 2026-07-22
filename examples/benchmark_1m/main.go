package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	zvec "github.com/third-apps/go-zvec"
	"github.com/third-apps/go-zvec/collection"
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

const (
	dim      = 128
	numDocs  = 1000000
	topK     = 10
	numQuery = 5000
	batchSz  = 20000
)

func main() {
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("============================================================")
	fmt.Println("  Go-Zvec 1M 向量大规模测试 / 1M Vector Large-Scale Benchmark")
	fmt.Println("============================================================")
	fmt.Printf("维度: %d | 文档数: %d | TopK: %d | 查询数: %d\n", dim, numDocs, topK, numQuery)
	fmt.Printf("Go: %s | CPU: %d | OS: %s\n", runtime.Version(), runtime.NumCPU(), runtime.GOOS)
	fmt.Println("============================================================")

	fmt.Println("\n生成向量中...")
	vectors := generateVectors(numDocs, dim)
	queryVecs := generateVectors(numQuery, dim)
	fmt.Println("向量生成完成")

	fmt.Println("\n--- HNSW RaBitQ (1-bit 量化) ---")
	benchHNSWRabitq(vectors, queryVecs)

	fmt.Println("\n--- HNSW (标准) ---")
	benchHNSW(vectors, queryVecs)

	// Vamana 插入 1M 较慢，取消注释后运行
	fmt.Println("\n--- Vamana ---")
	benchVamana(vectors, queryVecs)

	// Flat 暴力搜索 1M 较慢，取消注释后运行
	fmt.Println("\n--- Flat ---")
	benchFlat(vectors, queryVecs)

	// Recall 计算（对 1M 暴力搜索 ground truth 较慢），取消注释后运行
	fmt.Println("\n--- Recall 测试 (50 queries) ---")
	benchRecall(vectors, queryVecs)

	fmt.Println("\n============================================================")
	fmt.Println("  大规模测试完成 / Large-Scale Benchmark Complete")
	fmt.Println("============================================================")
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

func createColl(name string, params param.IndexConfig) (*collection.Collection, string) {
	dir := filepath.Join(os.TempDir(), "go-zvec-1m-"+name)
	os.RemoveAll(dir)

	s := schema.NewCollectionSchema(name)
	f := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, dim)
	f.SetIndexParams(params)
	s.AddField(f)

	c, err := zvec.CreateAndOpen(dir, s, &collection.Options{})
	if err != nil {
		log.Fatalf("[%s] 创建集合失败: %v", name, err)
	}
	return c, dir
}

func insertData(c *collection.Collection, name string, vectors [][]float32) int64 {
	start := time.Now()
	for i := 0; i < len(vectors); i += batchSz {
		end := i + batchSz
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
			log.Fatalf("[%s] 插入失败: %v", name, st.Error())
		}
		if (i+batchSz)%200000 == 0 {
			elapsed := time.Since(start)
			fmt.Printf("  [%s] 已插入 %d / %d (%.0f docs/s)\n", name, i+batchSz, len(vectors),
				float64(i+batchSz)/elapsed.Seconds())
		}
	}
	return time.Since(start).Milliseconds()
}

func searchBench(c *collection.Collection, name string, queryVecs [][]float32) (float64, float64, float64, float64, float64) {
	latencies := make([]float64, len(queryVecs))
	start := time.Now()
	for i, qv := range queryVecs {
		qStart := time.Now()
		c.Query(&query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: qv},
			},
			TopK: topK,
		})
		latencies[i] = float64(time.Since(qStart).Nanoseconds()) / 1e6
	}
	elapsed := time.Since(start)
	qps := float64(len(queryVecs)) / elapsed.Seconds()
	avgLat := float64(elapsed.Milliseconds()) / float64(len(queryVecs))

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)
	p50 := percentile(sorted, 50)
	p95 := percentile(sorted, 95)
	p99 := percentile(sorted, 99)

	return qps, avgLat, p50, p95, p99
}

func printBenchResult(name string, insertMs int64, qps, avgLat, p50, p95, p99 float64, memBytes uint64) {
	fmt.Printf("  [%s] 插入: %d ms (%.0f docs/s)\n", name, insertMs, float64(numDocs)/float64(insertMs)*1000)
	fmt.Printf("  [%s] QPS: %.0f | 平均延迟: %.3f ms\n", name, qps, avgLat)
	fmt.Printf("  [%s] P50: %.3f ms | P95: %.3f ms | P99: %.3f ms\n", name, p50, p95, p99)
	fmt.Printf("  [%s] 内存: %.1f MB\n", name, float64(memBytes)/1024/1024)
}

func benchHNSW(vectors, queryVecs [][]float32) {
	c, dir := createColl("hnsw", param.NewHNSWIndexParams(types.MetricTypeCosine, 32, 400))
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertData(c, "HNSW", vectors)
	qps, avgLat, p50, p95, p99 := searchBench(c, "HNSW", queryVecs)
	printBenchResult("HNSW", insertMs, qps, avgLat, p50, p95, p99, c.Stats().TotalMemoryBytes)
}

func benchHNSWRabitq(vectors, queryVecs [][]float32) {
	c, dir := createColl("hnsw_rabitq", param.NewHNSWRabitqIndexParams(types.MetricTypeCosine, 4, 256, 32, 400, 1000))
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertData(c, "RaBitQ", vectors)
	qps, avgLat, p50, p95, p99 := searchBench(c, "RaBitQ", queryVecs)
	printBenchResult("RaBitQ", insertMs, qps, avgLat, p50, p95, p99, c.Stats().TotalMemoryBytes)
}

func benchVamana(vectors, queryVecs [][]float32) {
	c, dir := createColl("vamana", param.NewVamanaIndexParams(types.MetricTypeCosine, 32, 100, 1.2, false, false))
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertData(c, "Vamana", vectors)
	qps, avgLat, p50, p95, p99 := searchBench(c, "Vamana", queryVecs)
	printBenchResult("Vamana", insertMs, qps, avgLat, p50, p95, p99, c.Stats().TotalMemoryBytes)
}

func benchFlat(vectors, queryVecs [][]float32) {
	c, dir := createColl("flat", param.NewFlatIndexParams(types.MetricTypeCosine))
	defer c.Close()
	defer os.RemoveAll(dir)

	insertMs := insertData(c, "Flat", vectors)
	qps, avgLat, p50, p95, p99 := searchBench(c, "Flat", queryVecs)
	printBenchResult("Flat", insertMs, qps, avgLat, p50, p95, p99, c.Stats().TotalMemoryBytes)
}

func benchRecall(vectors, queryVecs [][]float32) {
	distFn := metric.GetDistanceFunc(types.MetricTypeCosine)
	sampleSize := 100
	if sampleSize > len(queryVecs) {
		sampleSize = len(queryVecs)
	}

	indexConfigs := []struct {
		name   string
		params param.IndexConfig
	}{
		{"HNSW", param.NewHNSWIndexParams(types.MetricTypeCosine, 32, 400)},
		{"RaBitQ", param.NewHNSWRabitqIndexParams(types.MetricTypeCosine, 4, 256, 32, 400, 1000)},
		{"Vamana", param.NewVamanaIndexParams(types.MetricTypeCosine, 32, 100, 1.2, false, false)},
	}

	for _, cfg := range indexConfigs {
		c, dir := createColl("recall_"+cfg.name, cfg.params)
		insertData(c, cfg.name, vectors)

		var totalRecall float64
		for qi := 0; qi < sampleSize; qi++ {
			qv := queryVecs[qi]
			q := make([]float32, len(qv))
			copy(q, qv)
			metric.NormalizeInPlace(q)

			results, _ := c.Query(&query.SearchQuery{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: qv},
				},
				TopK: topK,
			})

			annSet := make(map[string]struct{})
			for _, r := range results {
				if id, ok := r["id"]; ok {
					annSet[id.(string)] = struct{}{}
				}
			}

			type item struct {
				pk   string
				dist float32
			}
			items := make([]item, len(vectors))
			for i, v := range vectors {
				vn := make([]float32, len(v))
				copy(vn, v)
				metric.NormalizeInPlace(vn)
				items[i] = item{pk: fmt.Sprintf("d%d", i), dist: distFn(q, vn)}
			}
			sort.Slice(items, func(i, j int) bool { return items[i].dist < items[j].dist })

			k := topK
			if k > len(items) {
				k = len(items)
			}
			hits := 0
			for i := 0; i < k; i++ {
				if _, ok := annSet[items[i].pk]; ok {
					hits++
				}
			}
			totalRecall += float64(hits) / float64(k)
		}

		avgRecall := totalRecall / float64(sampleSize)
		fmt.Printf("  [%s] Recall@%d = %.4f (%.1f%%)\n", cfg.name, topK, avgRecall, avgRecall*100)

		c.Close()
		os.RemoveAll(dir)
	}
}
