package collection

import (
	"os"
	"testing"

	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

func testSchema() *schema.CollectionSchema {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("title", types.DataTypeString, true, 0))
	s.AddField(schema.NewFieldSchema("age", types.DataTypeInt32, true, 0))
	vecField := schema.NewFieldSchema("embedding", types.DataTypeVectorFP32, false, 4)
	vecField.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
	s.AddField(vecField)
	return s
}

// TestCreateAndOpen 验证 Collection 创建并打开后路径正确
func TestCreateAndOpen(t *testing.T) {
	path := "./test_zvec"
	defer os.RemoveAll(path)

	c, err := CreateAndOpen(path, testSchema(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if c.Path() != path {
		t.Fatalf("expected path %s, got %s", path, c.Path())
	}
}

// TestInsertAndQuery 验证 Collection 插入文档后向量搜索返回正确结果
func TestInsertAndQuery(t *testing.T) {
	path := "./test_zvec_insert"
	defer os.RemoveAll(path)

	c, err := CreateAndOpen(path, testSchema(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	d1 := doc.NewDoc("doc_1")
	d1.SetVectorFP32Field("embedding", []float32{0.1, 0.2, 0.3, 0.4})
	d2 := doc.NewDoc("doc_2")
	d2.SetVectorFP32Field("embedding", []float32{0.2, 0.3, 0.4, 0.1})

	st := c.Insert([]*doc.Doc{d1, d2})
	if !st.OK() {
		t.Fatal(st.Error())
	}

	if c.Stats().DocCount != 2 {
		t.Fatalf("expected 2 docs, got %d", c.Stats().DocCount)
	}

	results, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "embedding",
			Vector:    &query.VectorClause{QueryVector: []float32{0.4, 0.3, 0.3, 0.1}},
		},
		TopK: 5,
	})
	if !st.OK() {
		t.Fatal(st.Error())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestUpsert 验证 Collection 更新插入文档后文档数不变
func TestUpsert(t *testing.T) {
	path := "./test_zvec_upsert"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	d1 := doc.NewDoc("doc_1")
	d1.SetVectorFP32Field("embedding", []float32{0.1, 0.2, 0.3, 0.4})
	c.Insert([]*doc.Doc{d1})

	d1Updated := doc.NewDoc("doc_1")
	d1Updated.SetVectorFP32Field("embedding", []float32{0.9, 0.9, 0.9, 0.9})
	st := c.Upsert([]*doc.Doc{d1Updated})
	if !st.OK() {
		t.Fatal(st.Error())
	}

	if c.Stats().DocCount != 1 {
		t.Fatalf("expected 1 doc after upsert, got %d", c.Stats().DocCount)
	}
}

// TestDelete 验证 Collection 删除文档后文档数为0
func TestDelete(t *testing.T) {
	path := "./test_zvec_delete"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	c.Insert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetVectorFP32Field("embedding", []float32{0.1, 0.2, 0.3, 0.4})
			return d
		}(),
	})

	st := c.Delete([]string{"doc_1"})
	if !st.OK() {
		t.Fatal(st.Error())
	}
	if c.Stats().DocCount != 0 {
		t.Fatalf("expected 0 docs after delete, got %d", c.Stats().DocCount)
	}
}

// TestCreateIndex 验证 Collection 动态创建索引功能
func TestCreateIndex(t *testing.T) {
	path := "./test_zvec_create_idx"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	s.AddField(vecField)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	flatParams := param.NewFlatIndexParams(types.MetricTypeL2)
	st := c.CreateIndex("vec", flatParams)
	if !st.OK() {
		t.Fatal(st.Error())
	}
}

// TestFlush 验证 Collection 刷盘操作
func TestFlush(t *testing.T) {
	path := "./test_zvec_flush"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
}

// TestFetch 验证 Collection 按 PK 获取文档功能
func TestFetch(t *testing.T) {
	path := "./test_zvec_fetch"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	d1 := doc.NewDoc("doc_1")
	d1.SetVectorFP32Field("embedding", []float32{0.1, 0.2, 0.3, 0.4})
	c.Insert([]*doc.Doc{d1})

	fetched, st := c.Fetch([]string{"doc_1"}, nil, true)
	if !st.OK() {
		t.Fatal(st.Error())
	}
	if _, ok := fetched["doc_1"]; !ok {
		t.Fatal("expected to fetch doc_1")
	}
}
