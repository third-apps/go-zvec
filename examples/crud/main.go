// examples/crud — 演示集合的完整增删改查(CRUD)生命周期
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
	// 初始化引擎
	if err := zvec.Init(); err != nil {
		log.Fatal(err)
	}
	defer zvec.Shutdown()

	fmt.Println("=== CRUD 完整操作演示 ===")

	// 1. 创建 Schema：定义集合包含哪些字段
	s := schema.NewCollectionSchema("crud_demo")
	// 添加向量字段：2 维浮点向量，使用 HNSW 索引（余弦距离）
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	s.AddField(vecField)
	// 添加标量字段：整数类型的年龄
	ageField := schema.NewFieldSchema("age", types.DataTypeInt32, false, 0)
	s.AddField(ageField)
	// 添加标量字段：字符串类型的名称
	nameField := schema.NewFieldSchema("name", types.DataTypeString, false, 0)
	s.AddField(nameField)

	// 2. 创建集合（如果路径已存在会报错，请先清理目录）
	fmt.Println(">> 创建集合...")
	c, err := zvec.CreateAndOpen("./examples_crud", s, &collection.Options{})
	if err != nil {
		log.Fatal("创建集合失败:", err)
	}
	defer c.Close()

	// ============================================================
	// 3. 插入文档 (Create)
	// ============================================================
	fmt.Println(">> 插入文档...")
	docs := []*doc.Doc{
		createDoc("user_1", []float32{0.1, 0.2}, 25, "张三"),
		createDoc("user_2", []float32{0.3, 0.4}, 30, "李四"),
		createDoc("user_3", []float32{0.5, 0.6}, 22, "王五"),
		createDoc("user_4", []float32{0.7, 0.8}, 28, "赵六"),
	}
	if st := c.Insert(docs); !st.OK() {
		log.Fatal("插入失败:", st.Error())
	}
	fmt.Printf("已插入 %d 条文档\n", len(docs))

	// ============================================================
	// 4. 查询文档 (Read) — 向量相似度搜索
	// ============================================================
	fmt.Println(">> 向量最近邻搜索...")
	queryResults, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.15, 0.15}},
		},
		TopK: 3, // 返回最相似的 3 条
	})
	if !st.OK() {
		log.Fatal("查询失败:", st.Error())
	}
	for _, r := range queryResults {
		fmt.Printf("  命中: id=%s score=%.4f\n", r["id"], r["score"])
	}

	// ============================================================
	// 5. 更新文档 (Update) — 替换整个文档，重建向量索引
	// ============================================================
	fmt.Println(">> 更新文档 user_1...")
	updated := doc.NewDoc("user_1")
	updated.SetVectorFP32Field("vec", []float32{0.9, 0.9}) // 更新向量
	updated.SetInt32Field("age", 26)                       // 更新年龄
	updated.SetStringField("name", "张三(已更新)")
	if st := c.Update([]*doc.Doc{updated}); !st.OK() {
		log.Fatal("更新失败:", st.Error())
	}
	fmt.Println("更新成功")

	// ============================================================
	// 6. 覆盖插入 (Upsert) — 存在则更新，不存在则插入
	// ============================================================
	fmt.Println(">> Upsert 文档 user_2(更新) 和 user_5(新建)...")
	upsertUser2 := doc.NewDoc("user_2")
	upsertUser2.SetVectorFP32Field("vec", []float32{0.99, 0.01})
	upsertUser2.SetInt32Field("age", 31)
	upsertUser2.SetStringField("name", "李四(已更新)")

	upsertUser5 := doc.NewDoc("user_5")
	upsertUser5.SetVectorFP32Field("vec", []float32{0.4, 0.5})
	upsertUser5.SetInt32Field("age", 35)
	upsertUser5.SetStringField("name", "陈七")
	if st := c.Upsert([]*doc.Doc{upsertUser2, upsertUser5}); !st.OK() {
		log.Fatal("Upsert 失败:", st.Error())
	}
	fmt.Println("Upsert 成功")

	// ============================================================
	// 7. 精确查询 (Fetch) — 按主键 ID 获取文档
	// ============================================================
	fmt.Println(">> 精确查询 user_1 和 user_5...")
	fetched, st := c.Fetch([]string{"user_1", "user_5"}, nil, true)
	if !st.OK() {
		log.Fatal("Fetch 失败:", st.Error())
	}
	for id, d := range fetched {
		name, _ := d.Field("name")
		age, _ := d.Field("age")
		vec, _ := d.Vector("vec")
		fmt.Printf("  获取到: id=%s name=%v age=%v vec=%v\n", id, name, age, vec.Float32s)
	}

	// ============================================================
	// 8. 删除文档 (Delete) — 按主键删除
	// ============================================================
	fmt.Println(">> 删除文档 user_3...")
	if st := c.Delete([]string{"user_3"}); !st.OK() {
		log.Fatal("删除失败:", st.Error())
	}

	// 删除后再查询，确认 user_3 已消失
	afterDel, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{0.5, 0.5}},
		},
		TopK: 10,
	})
	if !st.OK() {
		log.Fatal("查询失败:", st.Error())
	}
	fmt.Printf("删除后搜索结果 (%d 条):\n", len(afterDel))
	for _, r := range afterDel {
		fmt.Printf("  id=%s score=%.4f\n", r["id"], r["score"])
	}

	// ============================================================
	// 9. 集合统计信息
	// ============================================================
	stats := c.Stats()
	fmt.Printf("集合统计: 文档数=%d\n", stats.DocCount)

	fmt.Println("=== CRUD 演示结束 ===")
}

// createDoc 创建一个包含向量+标量字段的文档
func createDoc(id string, vec []float32, age int32, name string) *doc.Doc {
	d := doc.NewDoc(id)
	d.SetVectorFP32Field("vec", vec)
	d.SetInt32Field("age", age)
	d.SetStringField("name", name)
	return d
}
