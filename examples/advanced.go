package main

import (
	"fmt"
	"log"

	zvec "github.com/third-apps/go-zvec"
	"github.com/third-apps/go-zvec/collection"
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

func main() {
	// 初始化引擎，设置全局配置
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("=== 多种索引类型演示 / Demo: Multiple Index Types ===")

	demoFlat()
	demoHNSW()
	demoIVF()
	demoVamana()
	demoFTS()
	demoHybridSearch()
}

// demoFlat 演示暴力搜索(Flat)索引：适用于小规模数据集，精确但不高效
func demoFlat() {
	fmt.Println("\n--- FLAT 索引 (暴力搜索) ---")
	// 创建集合 schema，定义向量字段（2维）
	s := schema.NewCollectionSchema("flat_demo")
	vecField := schema.NewFieldSchema("v", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(vecField)

	// 创建集合，指定存储路径
	c, err := zvec.CreateAndOpen("./demo_flat_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	addDemoDocs(c)
	queryDemo(c)
}

// demoHNSW 演示 HNSW 图索引：适用于大规模近似最近邻搜索
func demoHNSW() {
	fmt.Println("\n--- HNSW 索引 (分层可导航小世界图) ---")
	// HNSW 参数：M=16（每层最大连接数），EF=200（构建时搜索范围）
	s := schema.NewCollectionSchema("hnsw_demo")
	f := schema.NewFieldSchema("v", types.DataTypeVectorFP32, false, 2)
	f.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	s.AddField(f)

	c, err := zvec.CreateAndOpen("./demo_hnsw_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	addDemoDocs(c)
	queryDemo(c)
}

// demoIVF 演示 IVF 倒排文件索引：使用 k-means 聚类划分空间
func demoIVF() {
	fmt.Println("\n--- IVF 索引 (倒排文件) ---")
	// IVF 参数：NList=10（聚类中心数），NIters=20（训练迭代次数）
	s := schema.NewCollectionSchema("ivf_demo")
	f := schema.NewFieldSchema("v", types.DataTypeVectorFP32, false, 2)
	f.SetIndexParams(param.NewIVFIndexParams(types.MetricTypeL2, 10, 20, false))
	s.AddField(f)

	c, err := zvec.CreateAndOpen("./demo_ivf_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// 插入 50 条数据，IVF 需要较多数据训练聚类
	for i := 0; i < 50; i++ {
		d := doc.NewDoc(fmt.Sprintf("doc_%d", i))
		d.SetVectorFP32Field("v", []float32{float32(i) / 50, float32(50-i) / 50})
		st := c.Insert([]*doc.Doc{d})
		if !st.OK() {
			log.Fatal(st.Error())
		}
	}

	// 搜索与 [0.5, 0.5] 最近的 3 个向量
	results, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "v",
			Vector:    &query.VectorClause{QueryVector: []float32{0.5, 0.5}},
		},
		TopK: 3,
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("IVF 搜索结果: %d 条\n", len(results))
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}
}

// demoVamana 演示 Vamana 图索引：DiskANN 的核心算法，支持过滤搜索
func demoVamana() {
	fmt.Println("\n--- Vamana 索引 (过滤图索引) ---")
	// Vamana 参数：maxDegree=4（最大度数），searchList=20（搜索列表大小），alpha=1.2（剪枝参数）
	s := schema.NewCollectionSchema("vamana_demo")
	f := schema.NewFieldSchema("v", types.DataTypeVectorFP32, false, 2)
	f.SetIndexParams(param.NewVamanaIndexParams(types.MetricTypeCosine, 4, 20, 1.2, false, false))
	s.AddField(f)

	c, err := zvec.CreateAndOpen("./demo_vamana_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	addDemoDocs(c)
	queryDemo(c)
}

// demoFTS 演示全文搜索(Full-Text Search)：使用 BM25 评分和 Standard 分词器
func demoFTS() {
	fmt.Println("\n--- 全文搜索 FTS (BM25) ---")
	s := schema.NewCollectionSchema("fts_demo")
	f := schema.NewFieldSchema("text", types.DataTypeString, false, 0)
	// FTS 参数：使用 Standard 分词器（按空格+标点分词）
	f.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(f)

	c, err := zvec.CreateAndOpen("./demo_fts_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// 插入英文文本数据，FTS 会自动构建倒排索引
	texts := []string{
		"the quick brown fox jumps over the lazy dog",
		"a quick brown dog jumps over the lazy fox",
		"the lazy cat sleeps all day long",
		"the quick rabbit runs through the forest",
	}

	for i, text := range texts {
		d := doc.NewDoc(fmt.Sprintf("doc_%d", i))
		d.SetStringField("text", text)
		st := c.Insert([]*doc.Doc{d})
		if !st.OK() {
			log.Fatal(st.Error())
		}
	}

	// 搜索包含 "quick brown fox" 的文档
	results, st := c.FTSQuery("text", "quick brown fox", 5)
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("FTS 搜索 'quick brown fox' 的结果:\n")
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f text=%s\n", r["id"], r["score"], r["text"])
	}
}

// demoHybridSearch 演示混合搜索：向量搜索 + 全文搜索 + RRF 重排序融合
func demoHybridSearch() {
	fmt.Println("\n--- 混合搜索 (向量 + 全文搜索 + RRF 融合) ---")
	// 定义多字段 Schema：向量字段(embedding) + 文本字段(title) + FTS索引
	s := schema.NewCollectionSchema("hybrid_demo")
	vecF := schema.NewFieldSchema("embedding", types.DataTypeVectorFP32, false, 2)
	vecF.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	s.AddField(vecF)
	ftsF := schema.NewFieldSchema("title", types.DataTypeString, false, 0)
	ftsF.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(ftsF)

	c, err := zvec.CreateAndOpen("./demo_hybrid_data", s, &collection.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// 插入混合数据：每条记录同时包含向量和文本
	docs := []struct {
		id    string
		vec   []float32
		title string
	}{
		{"doc_1", []float32{0.1, 0.2}, "quick brown fox"},
		{"doc_2", []float32{0.2, 0.1}, "lazy dog"},
		{"doc_3", []float32{0.9, 0.8}, "brown bear"},
	}

	for _, d := range docs {
		zd := doc.NewDoc(d.id)
		zd.SetVectorFP32Field("embedding", d.vec)
		zd.SetStringField("title", d.title)
		c.Insert([]*doc.Doc{zd})
	}

	// 使用 MultiQuery + RRF 重排序进行混合搜索
	mq := &query.MultiQuery{
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: "embedding",
					Vector:    &query.VectorClause{QueryVector: []float32{0.15, 0.15}},
				},
				NumCandidates: 5, // 每个子查询取前 5 个候选
			},
		},
		TopK: 3,
		Rerank: query.RerankParams{
			Type:        query.RerankTypeRRF, // 使用倒数秩融合(RRF)
			RRFConstant: 60,                  // RRF 常数 k=60
		},
	}

	results, st := c.MultiQuery(mq)
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("混合搜索 (向量+RRF) 结果:\n")
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// 同时对文本字段执行 FTS 搜索
	vResults, st := c.FTSQuery("title", "brown", 5)
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("FTS 搜索 'brown' 的结果:\n")
	for _, r := range vResults {
		fmt.Printf("  id=%s score=%.4f title=%s\n", r["id"], r["score"], r["text"])
	}
}

// addDemoDocs 向集合中插入示例数据
func addDemoDocs(c *collection.Collection) {
	docData := []struct {
		id  string
		vec []float32
	}{
		{"doc_1", []float32{0.1, 0.2}},
		{"doc_2", []float32{0.2, 0.1}},
		{"doc_3", []float32{0.9, 0.8}},
	}

	for _, d := range docData {
		zd := doc.NewDoc(d.id)
		zd.SetVectorFP32Field("v", d.vec)
		st := c.Insert([]*doc.Doc{zd})
		if !st.OK() {
			log.Fatal(st.Error())
		}
	}
}

// queryDemo 对集合执行最近邻查询并打印结果
func queryDemo(c *collection.Collection) {
	results, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "v",
			Vector:    &query.VectorClause{QueryVector: []float32{0.15, 0.15}},
		},
		TopK: 3,
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("搜索结果: %d 条\n", len(results))
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}
}
