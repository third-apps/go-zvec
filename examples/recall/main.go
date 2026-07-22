package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

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
	numDocs  = 10000
	topK     = 10
	numQuery = 50
)

func main() {
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("============================================")
	fmt.Println("  Go-Zvec Recall Benchmark")
	fmt.Println("============================================")
	fmt.Printf("维度: %d | 文档数: %d | TopK: %d | 查询数: %d\n", dim, numDocs, topK, numQuery)
	fmt.Println("============================================")

	vectors := generateVectors(numDocs, dim)
	queryVecs := generateVectors(numQuery, dim)

	groundTruth := buildGroundTruth(vectors, queryVecs)

	measureRecall("HNSW (M=16, efConstruction=200)",
		param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200),
		vectors, queryVecs, groundTruth)

	measureRecall("Vamana (maxDegree=16, alpha=1.2)",
		param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false),
		vectors, queryVecs, groundTruth)

	measureRecall("IVF (nList=16, nIters=20)",
		param.NewIVFIndexParams(types.MetricTypeCosine, 16, 20, false),
		vectors, queryVecs, groundTruth)

	fmt.Println("\n============================================")
	fmt.Println("  Recall Benchmark Complete")
	fmt.Println("============================================")
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

func buildGroundTruth(vectors, queryVecs [][]float32) [][]string {
	distFn := metric.GetDistanceFunc(types.MetricTypeCosine)
	truth := make([][]string, len(queryVecs))

	for qi, qv := range queryVecs {
		q := make([]float32, len(qv))
		copy(q, qv)
		metric.NormalizeInPlace(q)

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
		truth[qi] = make([]string, k)
		for i := 0; i < k; i++ {
			truth[qi][i] = items[i].pk
		}
	}
	return truth
}

func measureRecall(name string, indexParams *param.IndexParams, vectors, queryVecs [][]float32, groundTruth [][]string) {
	dir := filepath.Join(os.TempDir(), "go-zvec-recall-"+name)
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	s := schema.NewCollectionSchema("recall_" + name)
	f := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, dim)
	f.SetIndexParams(indexParams)
	s.AddField(f)

	c, err := zvec.CreateAndOpen(dir, s, &collection.Options{})
	if err != nil {
		log.Fatalf("[%s] 创建集合失败: %v", name, err)
	}
	defer c.Close()

	for i, v := range vectors {
		d := doc.NewDoc(fmt.Sprintf("d%d", i))
		d.SetVectorFP32Field("vec", v)
		if st := c.Insert([]*doc.Doc{d}); !st.OK() {
			log.Fatalf("[%s] 插入失败: %v", name, st.Error())
		}
	}

	var totalRecall float64
	for qi, qv := range queryVecs {
		results, st := c.Query(&query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: qv},
			},
			TopK: topK,
		})
		if !st.OK() {
			log.Fatalf("[%s] 查询失败: %v", name, st.Error())
		}

		annSet := make(map[string]struct{})
		for _, r := range results {
			if id, ok := r["id"]; ok {
				annSet[id.(string)] = struct{}{}
			}
		}

		hits := 0
		for _, pk := range groundTruth[qi] {
			if _, ok := annSet[pk]; ok {
				hits++
			}
		}
		totalRecall += float64(hits) / float64(len(groundTruth[qi]))
	}

	avgRecall := totalRecall / float64(len(queryVecs))
	fmt.Printf("[%s] Recall@%d = %.4f (%.1f%%)\n", name, topK, avgRecall, avgRecall*100)
}
