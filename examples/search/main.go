// examples/search — 演示各种搜索方式：向量搜索、过滤搜索、全文搜索、多查询融合、分组聚合
package main

import (
	"fmt"
	"log"
	"os"

	zvec "github.com/third-apps/go-zvec"
	"github.com/third-apps/go-zvec/collection"
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

func main() {
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("=== 多种搜索方式演示 ===")

	// ---------- 准备测试数据 ----------
	fmt.Println(">> 创建集合并插入数据...")
	s := schema.NewCollectionSchema("search_demo")
	// 向量字段：用于 ANN 搜索
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	s.AddField(vecField)
	// 分类字段：用于过滤
	catField := schema.NewFieldSchema("category", types.DataTypeString, false, 0)
	s.AddField(catField)
	// 价格字段：用于范围过滤
	priceField := schema.NewFieldSchema("price", types.DataTypeFloat, false, 0)
	s.AddField(priceField)
	// 文本字段：用于全文搜索
	ftsField := schema.NewFieldSchema("content", types.DataTypeString, false, 0)
	ftsField.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(ftsField)

	// 使用临时目录，避免路径冲突
	cleanDir("./search_demo_data")
	c, err := zvec.CreateAndOpen("./search_demo_data", s, &collection.Options{})
	if err != nil {
		log.Fatal("创建集合失败:", err)
	}
	defer c.Close()

	// 插入模拟电商数据
	products := []struct {
		id       string
		vec      []float32
		category string
		price    float32
		content  string
	}{
		{"p_1", []float32{0.1, 0.9}, "电子产品", 5999, "最新款智能手机，高性能处理器"},
		{"p_2", []float32{0.2, 0.8}, "电子产品", 12999, "轻薄笔记本电脑，适合办公"},
		{"p_3", []float32{0.8, 0.2}, "家居用品", 299, "智能台灯，护眼调光"},
		{"p_4", []float32{0.7, 0.3}, "家居用品", 899, "人体工学椅，舒适靠背"},
		{"p_5", []float32{0.4, 0.6}, "服饰", 199, "纯棉T恤，透气面料"},
		{"p_6", []float32{0.5, 0.5}, "服饰", 399, "运动鞋，缓震防滑"},
		{"p_7", []float32{0.9, 0.1}, "食品", 59, "进口巧克力礼盒"},
		{"p_8", []float32{0.3, 0.7}, "食品", 29, "有机坚果混合装"},
	}

	for _, p := range products {
		d := doc.NewDoc(p.id)
		d.SetVectorFP32Field("vec", p.vec)
		d.SetStringField("category", p.category)
		d.SetFloatField("price", p.price)
		d.SetStringField("content", p.content)
		if st := c.Insert([]*doc.Doc{d}); !st.OK() {
			log.Fatal("插入失败:", st.Error())
		}
	}
	fmt.Println("数据插入完成")

	// ============================================================
	// 1. 基础向量搜索 — 返回最近的 TopK 条
	// ============================================================
	fmt.Println("\n--- 1. 基础向量搜索 ---")
	results, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.15, 0.85}},
		},
		TopK: 3, // 返回最相似的 3 条
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// ============================================================
	// 2. 过滤搜索 — 使用 Filter 表达式缩小搜索范围
	// ============================================================
	fmt.Println("\n--- 2. 过滤搜索 (只看家居用品) ---")
	results, st = c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.15, 0.85}},
		},
		Filter: "category=家居用品", // 过滤表达式：category 等于 "家居用品"
		TopK:   10,
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// 支持多种比较操作符
	fmt.Println("\n--- 过滤搜索 (价格 >= 500) ---")
	results, st = c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.5, 0.5}},
		},
		Filter: "price>=500", // >= (大于等于)、<=、>、<、==、!= 均支持
		TopK:   10,
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, r := range results {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// ============================================================
	// 3. 全文搜索 (FTS) — 按文本相关性搜索
	// ============================================================
	fmt.Println("\n--- 3. 全文搜索 (搜索 '电脑 处理器') ---")
	ftsResults, st := c.FTSQuery("content", "电脑 处理器", 5)
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, r := range ftsResults {
		fmt.Printf("  id=%s score=%.4f text=%s\n", r["id"], r["score"], r["text"])
	}

	// ============================================================
	// 4. 多查询融合 (MultiQuery) — 多路召回 + RRF/WRR 重排序
	// ============================================================
	fmt.Println("\n--- 4. 多查询融合 (向量 + 向量 + RRF 重排序) ---")
	mqResults, st := c.MultiQuery(&query.MultiQuery{
		// 子查询 1：搜索 vec 字段，用第一个向量
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: []float32{0.1, 0.9}},
				},
				NumCandidates: 10, // 每路召回前 10 条
			},
			// 子查询 2：搜索 vec 字段，用第二个向量
			{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: []float32{0.8, 0.2}},
				},
				NumCandidates: 10,
			},
		},
		TopK: 5, // 最终返回 5 条
		Rerank: query.RerankParams{
			Type:        query.RerankTypeRRF, // RRF (倒数秩融合) 或 RerankTypeWeighted (加权)
			RRFConstant: 60,                  // RRF 常数 k，越大越平滑
		},
		OutputFields: []string{}, // 返回额外字段（留空只返回 id 和 score）
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, r := range mqResults {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// ============================================================
	// 5. 分组聚合搜索 (GroupBy) — 按字段分组返回 TopK 结果
	// ============================================================
	fmt.Println("\n--- 5. 分组搜索 (按 category 分组，每组最相似的 1 条) ---")
	gbResults, st := c.GroupBy(&query.GroupByVectorQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.4, 0.5}},
		},
		GroupByField:  "category", // 按分类字段分组
		TopKPerGroup:  1,          // 每组返回 1 条
		GroupCount:    10,         // 最多 10 组
		IncludeVector: false,
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	for _, g := range gbResults {
		fmt.Printf("  分组: %s\n", g.GroupByValue)
		for _, d := range g.Docs {
			fmt.Printf("    文档: %v\n", d)
		}
	}

	// ============================================================
	// 6. 带过滤的多查询 — 在每路子查询中应用过滤条件
	// ============================================================
	fmt.Println("\n--- 6. 带过滤的多查询 (只看价格 < 1000 的商品) ---")
	filteredMQ, st := c.MultiQuery(&query.MultiQuery{
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: []float32{0.3, 0.7}},
				},
				NumCandidates: 10,
			},
		},
		TopK:   5,
		Filter: "price<1000", // 全局过滤条件
		Rerank: query.RerankParams{
			Type:        query.RerankTypeRRF,
			RRFConstant: 60,
		},
	})
	if !st.OK() {
		log.Fatal(st.Error())
	}
	fmt.Printf("过滤后结果: %d 条\n", len(filteredMQ))
	for _, r := range filteredMQ {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	fmt.Println("\n=== 搜索演示结束 ===")
}

// cleanDir 删除目录用于演示
func cleanDir(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Fatal(err)
	}
}
