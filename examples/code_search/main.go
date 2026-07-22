package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
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
	dim        = 128
	numChunks  = 50000
	topK       = 10
	numQueries = 1000
	languages  = 6
	fileCount  = 200
)

var langNames = []string{"go", "python", "rust", "java", "typescript", "cpp"}

type CodeChunk struct {
	ID       string
	File     string
	Language string
	Function string
	Line     int
	Vector   []float32
}

func main() {
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("============================================")
	fmt.Println("  AI IDE 代码搜索场景模拟测试")
	fmt.Println("============================================")
	fmt.Printf("代码块数: %d | 维度: %d | TopK: %d\n", numChunks, dim, topK)
	fmt.Printf("语言: %v | 文件数: %d\n", langNames, fileCount)
	fmt.Println("============================================")

	chunks := generateCodeChunks()
	queryChunks := chunks[:numQueries]

	benchCodeSearch(chunks, queryChunks, "flat", param.NewFlatIndexParams(types.MetricTypeCosine))
	benchCodeSearch(chunks, queryChunks, "hnsw", param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	benchCodeSearch(chunks, queryChunks, "vamana", param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false))

	benchCodeSearchWithMetaFilter(chunks, queryChunks)

	fmt.Println("\n============================================")
	fmt.Println("  代码搜索场景测试完成")
	fmt.Println("============================================")
}

func generateCodeChunks() []*CodeChunk {
	rng := rand.New(rand.NewSource(42))
	chunks := make([]*CodeChunk, numChunks)

	for i := 0; i < numChunks; i++ {
		v := make([]float32, dim)
		var sum float32
		for j := 0; j < dim; j++ {
			v[j] = rng.Float32()
			sum += v[j] * v[j]
		}
		norm := float32(1.0 / float64(sum))
		for j := range v {
			v[j] *= norm
		}

		lang := langNames[i%languages]
		file := fmt.Sprintf("src/%s/module_%d/%s", lang, i%fileCount, lang)
		fn := fmt.Sprintf("func_%s_%d", lang, i)
		line := (i % 500) + 1

		chunks[i] = &CodeChunk{
			ID:       fmt.Sprintf("chunk_%d", i),
			File:     file,
			Language: lang,
			Function: fn,
			Line:     line,
			Vector:   v,
		}
	}
	return chunks
}

func benchCodeSearch(chunks []*CodeChunk, queryChunks []*CodeChunk, name string, indexParams *param.IndexParams) {
	dir := filepath.Join(os.TempDir(), "go-zvec-code-search-"+name)
	os.RemoveAll(dir)

	s := schema.NewCollectionSchema("code_search")
	f := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, dim)
	f.SetIndexParams(indexParams)
	s.AddField(f)

	c, err := zvec.CreateAndOpen(dir, s, &collection.Options{})
	if err != nil {
		log.Fatalf("[%s] 创建集合失败: %v", name, err)
	}
	defer c.Close()
	defer os.RemoveAll(dir)

	start := time.Now()
	for i := 0; i < len(chunks); i += 500 {
		end := i + 500
		if end > len(chunks) {
			end = len(chunks)
		}
		docs := make([]*doc.Doc, end-i)
		for j := i; j < end; j++ {
			d := doc.NewDoc(chunks[j].ID)
			d.SetVectorFP32Field("vec", chunks[j].Vector)
			docs[j-i] = d
		}
		if st := c.Insert(docs); !st.OK() {
			log.Fatalf("[%s] 插入失败: %v", name, st.Error())
		}
	}
	insertMs := time.Since(start).Milliseconds()

	start = time.Now()
	for i := 0; i < len(queryChunks); i++ {
		c.Query(&query.SearchQuery{
			Target: query.QueryTarget{
				FieldName: "vec",
				Vector:    &query.VectorClause{QueryVector: queryChunks[i].Vector},
			},
			TopK: topK,
		})
	}
	searchMs := time.Since(start).Milliseconds()
	qps := float64(len(queryChunks)) / (float64(searchMs) / 1000.0)

	stats := c.Stats()
	fmt.Printf("\n[%s]\n", name)
	fmt.Printf("  插入: %d ms | 搜索: %d ms | QPS: %.0f\n", insertMs, searchMs, qps)
	fmt.Printf("  内存: %.2f MB\n", float64(stats.TotalMemoryBytes)/1024/1024)
}

func benchCodeSearchWithMetaFilter(chunks []*CodeChunk, queryChunks []*CodeChunk) {
	dir := filepath.Join(os.TempDir(), "go-zvec-code-search-metafilter")
	os.RemoveAll(dir)

	s := schema.NewCollectionSchema("code_search_meta")
	f := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, dim)
	f.SetIndexParams(param.NewVamanaIndexParams(types.MetricTypeCosine, 16, 30, 1.2, false, false))
	s.AddField(f)

	langField := schema.NewFieldSchema("language", types.DataTypeString, false, 0)
	langField.SetIndexParams(param.NewInvertIndexParams(false, false))
	s.AddField(langField)

	fileField := schema.NewFieldSchema("file", types.DataTypeString, false, 0)
	s.AddField(fileField)

	c, err := zvec.CreateAndOpen(dir, s, &collection.Options{})
	if err != nil {
		log.Fatalf("创建集合失败: %v", err)
	}
	defer c.Close()
	defer os.RemoveAll(dir)

	start := time.Now()
	for i := 0; i < len(chunks); i += 500 {
		end := i + 500
		if end > len(chunks) {
			end = len(chunks)
		}
		docs := make([]*doc.Doc, end-i)
		for j := i; j < end; j++ {
			d := doc.NewDoc(chunks[j].ID)
			d.SetVectorFP32Field("vec", chunks[j].Vector)
			d.SetStringField("language", chunks[j].Language)
			d.SetStringField("file", chunks[j].File)
			docs[j-i] = d
		}
		if st := c.Insert(docs); !st.OK() {
			log.Fatalf("插入失败: %v", st.Error())
		}
	}
	insertMs := time.Since(start).Milliseconds()

	fmt.Printf("\n[Vamana + Metadata Filter]\n")
	fmt.Printf("  插入: %d ms\n", insertMs)

	scenarios := []struct {
		name   string
		filter *query.MetadataFilter
	}{
		{"无过滤", nil},
		{"language=go", query.NewMetadataFilter().WhereEq("language", "go")},
		{"language=python", query.NewMetadataFilter().WhereEq("language", "python")},
		{"language IN [go,rust]", query.NewMetadataFilter().WhereIn("language", []string{"go", "rust"})},
	}

	for _, sc := range scenarios {
		start = time.Now()
		for i := 0; i < len(queryChunks); i++ {
			c.Query(&query.SearchQuery{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: queryChunks[i].Vector},
				},
				TopK:       topK,
				MetaFilter: sc.filter,
			})
		}
		searchMs := time.Since(start).Milliseconds()
		qps := float64(len(queryChunks)) / (float64(searchMs) / 1000.0)
		fmt.Printf("  %s: %d ms | QPS: %.0f\n", sc.name, searchMs, qps)
	}

	fmt.Printf("\n  Metadata字段统计:\n")
	for _, lang := range langNames {
		fmt.Printf("    language=%s: %d chunks\n", lang, len(chunks)/languages)
	}

	files := make(map[string]int)
	for _, ch := range chunks {
		files[ch.File]++
	}
	counts := make([]int, 0, len(files))
	for _, c := range files {
		counts = append(counts, c)
	}
	sort.Ints(counts)
	fmt.Printf("    文件数: %d | 每文件chunk数: min=%d, max=%d, median=%d\n",
		len(files), counts[0], counts[len(counts)-1], counts[len(counts)/2])
}
