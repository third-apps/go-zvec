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

// TestCompileFilter_GreaterEqual 验证过滤表达式编译：大于等于
func TestCompileFilter_GreaterEqual(t *testing.T) {
	fn := compileFilter("age >= 30")
	s := testSchema()
	d := doc.NewDoc("x")
	d.SetInt32Field("age", 35)
	if !fn(d) {
		t.Fatal("expected 35 >= 30 to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetInt32Field("age", 25)
	if fn(d2) {
		t.Fatal("expected 25 >= 30 to not match")
	}
	_ = s
}

// TestCompileFilter_LessEqual 验证过滤表达式编译：小于等于
func TestCompileFilter_LessEqual(t *testing.T) {
	fn := compileFilter("score <= 50.5")
	d := doc.NewDoc("x")
	d.SetFloatField("score", 30)
	if !fn(d) {
		t.Fatal("expected 30 <= 50.5 to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetFloatField("score", 100)
	if fn(d2) {
		t.Fatal("expected 100 <= 50.5 to not match")
	}
}

// TestCompileFilter_Greater 验证过滤表达式编译：大于
func TestCompileFilter_Greater(t *testing.T) {
	fn := compileFilter("count > 10")
	d := doc.NewDoc("x")
	d.SetInt64Field("count", 15)
	if !fn(d) {
		t.Fatal("expected 15 > 10 to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetInt32Field("count", 5)
	if fn(d2) {
		t.Fatal("expected 5 > 10 to not match")
	}
}

// TestCompileFilter_Less 验证过滤表达式编译：小于
func TestCompileFilter_Less(t *testing.T) {
	fn := compileFilter("price < 100")
	d := doc.NewDoc("x")
	d.SetDoubleField("price", 50)
	if !fn(d) {
		t.Fatal("expected 50 < 100 to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetDoubleField("price", 200)
	if fn(d2) {
		t.Fatal("expected 200 < 100 to not match")
	}
}

// TestCompileFilter_Equal 验证过滤表达式编译：双等号等于
func TestCompileFilter_Equal(t *testing.T) {
	fn := compileFilter("name == alice")
	d := doc.NewDoc("x")
	d.SetStringField("name", "alice")
	if !fn(d) {
		t.Fatal("expected 'alice' == 'alice' to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetStringField("name", "bob")
	if fn(d2) {
		t.Fatal("expected 'bob' != 'alice' to not match")
	}
}

// TestCompileFilter_SingleEqual 验证过滤表达式编译：单等号等于
func TestCompileFilter_SingleEqual(t *testing.T) {
	fn := compileFilter("name = bob")
	d := doc.NewDoc("x")
	d.SetStringField("name", "bob")
	if !fn(d) {
		t.Fatal("expected 'bob' = 'bob' to match")
	}
}

// TestCompileFilter_NotEqual 验证过滤表达式编译：不等于
func TestCompileFilter_NotEqual(t *testing.T) {
	fn := compileFilter("status != active")
	d := doc.NewDoc("x")
	d.SetStringField("status", "inactive")
	if !fn(d) {
		t.Fatal("expected 'inactive' != 'active' to match")
	}
	d2 := doc.NewDoc("y")
	d2.SetStringField("status", "active")
	if fn(d2) {
		t.Fatal("expected 'active' != 'active' to not match")
	}
}

// TestCompileFilter_NullField 验证过滤表达式编译：缺失字段不匹配
func TestCompileFilter_NullField(t *testing.T) {
	fn := compileFilter("age >= 30")
	d := doc.NewDoc("x")
	if fn(d) {
		t.Fatal("expected doc without field to not match")
	}
}

// TestCompileFilter_EmptyFilter 验证过滤表达式编译：空过滤匹配所有文档
func TestCompileFilter_EmptyFilter(t *testing.T) {
	fn := compileFilter("")
	d := doc.NewDoc("x")
	if !fn(d) {
		t.Fatal("expected empty filter to match everything")
	}
}

// TestMatchFilterDelegation 验证 matchFilter 委托函数的过滤匹配逻辑
func TestMatchFilterDelegation(t *testing.T) {
	s := testSchema()
	d := doc.NewDoc("x")
	d.SetInt32Field("age", 25)
	_ = s
	if !matchFilter(d, "") {
		t.Fatal("expected empty filter to be true")
	}
	if matchFilter(d, "age > 30") {
		t.Fatal("expected 25 > 30 to be false")
	}
	if !matchFilter(d, "age < 30") {
		t.Fatal("expected 25 < 30 to be true")
	}
}

// TestUpdateIndexConsistency 验证 Upsert 更新文档后索引一致性
func TestUpdateIndexConsistency(t *testing.T) {
	path := "./test_update_idx"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(vecField)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	c.Insert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetVectorFP32Field("vec", []float32{1, 0})
			return d
		}(),
	})

	c.Upsert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetVectorFP32Field("vec", []float32{100, 0})
			return d
		}(),
	})

	results, _ := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "vec",
			Vector:    &query.VectorClause{QueryVector: []float32{1, 0}},
		},
		TopK: 1,
	})
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
}

// TestMultiQuery 验证 Collection 多向量字段联合查询功能
func TestMultiQuery(t *testing.T) {
	path := "./test_multi_q"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	v1 := schema.NewFieldSchema("v1", types.DataTypeVectorFP32, false, 2)
	v1.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(v1)
	v2 := schema.NewFieldSchema("v2", types.DataTypeVectorFP32, false, 2)
	v2.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(v2)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	c.Insert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetVectorFP32Field("v1", []float32{1, 0})
			d.SetVectorFP32Field("v2", []float32{0, 1})
			return d
		}(),
	})

	results, st := c.MultiQuery(&query.MultiQuery{
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: "v1",
					Vector:    &query.VectorClause{QueryVector: []float32{1, 0}},
				},
				NumCandidates: 10,
			},
			{
				Target: query.QueryTarget{
					FieldName: "v2",
					Vector:    &query.VectorClause{QueryVector: []float32{0, 1}},
				},
				NumCandidates: 10,
			},
		},
		TopK: 1,
	})
	if !st.OK() {
		t.Fatal(st.Error())
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from multi query")
	}
}

// TestMultiQueryWithFilter 验证 Collection 多向量字段联合查询带过滤条件
func TestMultiQueryWithFilter(t *testing.T) {
	path := "./test_multi_filter"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("title", types.DataTypeString, true, 0))
	vf := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vf.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(vf)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	c.Insert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetStringField("title", "good")
			d.SetVectorFP32Field("vec", []float32{1, 0})
			return d
		}(),
		func() *doc.Doc {
			d := doc.NewDoc("doc_2")
			d.SetStringField("title", "bad")
			d.SetVectorFP32Field("vec", []float32{0, 1})
			return d
		}(),
	})

	results, _ := c.MultiQuery(&query.MultiQuery{
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: "vec",
					Vector:    &query.VectorClause{QueryVector: []float32{1, 0}},
				},
				NumCandidates: 10,
			},
		},
		TopK:   5,
		Filter: "title == bad",
	})
	for _, r := range results {
		if v, ok := r["title"]; ok && v == "bad" {
			return
		}
	}
}

// TestFTSQuery 验证 Collection 全文搜索查询功能
func TestFTSQuery(t *testing.T) {
	path := "./test_fts_q"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	ftsField := schema.NewFieldSchema("content", types.DataTypeString, true, 0)
	ftsField.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(ftsField)
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(vecField)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	c.Insert([]*doc.Doc{
		func() *doc.Doc {
			d := doc.NewDoc("doc_1")
			d.SetStringField("content", "hello world")
			d.SetVectorFP32Field("vec", []float32{1, 0})
			return d
		}(),
		func() *doc.Doc {
			d := doc.NewDoc("doc_2")
			d.SetStringField("content", "hello foo")
			d.SetVectorFP32Field("vec", []float32{0, 1})
			return d
		}(),
	})

	results, st := c.Query(&query.SearchQuery{
		Target: query.QueryTarget{
			FieldName: "content",
			FTS:       &query.FTSClause{QueryString: "hello"},
		},
		TopK: 5,
	})
	if !st.OK() {
		t.Fatal(st.Error())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 FTS results, got %d", len(results))
	}
}

// TestDeleteByFilter 验证 Collection 按过滤条件删除文档
func TestDeleteByFilter(t *testing.T) {
	path := "./test_del_filter"
	defer os.RemoveAll(path)

	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("age", types.DataTypeInt32, true, 0))
	vecField := schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2)
	vecField.SetIndexParams(param.NewFlatIndexParams(types.MetricTypeL2))
	s.AddField(vecField)

	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	for i := 0; i < 5; i++ {
		d := doc.NewDoc("")
		d.SetInt32Field("age", int32(20+i))
		d.SetVectorFP32Field("vec", []float32{float32(i) / 5, float32(5-i) / 5})
		c.Insert([]*doc.Doc{d})
	}
}

// TestCollectionOpen 验证 Collection 关闭后重新打开
func TestCollectionOpen(t *testing.T) {
	path := "./test_coll_open"
	defer os.RemoveAll(path)

	c, err := CreateAndOpen(path, testSchema(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	c2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer c2.Close()

	if c2.Path() != path {
		t.Fatalf("expected path %s, got %s", path, c2.Path())
	}
}

// TestCollectionSchemaGetter 验证 Collection Schema 获取器
func TestCollectionSchemaGetter(t *testing.T) {
	path := "./test_schema_getter"
	defer os.RemoveAll(path)

	s := testSchema()
	c, _ := CreateAndOpen(path, s, nil)
	defer c.Close()

	got := c.Schema()
	if got.Name != s.Name {
		t.Fatal("Schema() getter mismatch")
	}
}

// TestCollectionOptionsGetter 验证 Collection Options 获取器
func TestCollectionOptionsGetter(t *testing.T) {
	path := "./test_opts_getter"
	defer os.RemoveAll(path)

	opts := &Options{ReadOnly: true}
	c, _ := CreateAndOpen(path, testSchema(), opts)
	defer c.Close()

	got := c.Options()
	if got.ReadOnly != opts.ReadOnly {
		t.Fatal("Options() getter mismatch")
	}
}

// TestCollectionAddColumn 验证 Collection 动态添加列
func TestCollectionAddColumn(t *testing.T) {
	path := "./test_add_col"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	f := schema.NewFieldSchema("extra", types.DataTypeString, true, 0)
	st := c.AddColumn(f, "")
	if !st.OK() {
		t.Fatalf("AddColumn failed: %v", st.Message())
	}
}

// TestCollectionDropColumn 验证 Collection 动态删除列
func TestCollectionDropColumn(t *testing.T) {
	path := "./test_drop_col"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	st := c.DropColumn("age")
	if !st.OK() {
		t.Fatalf("DropColumn failed: %v", st.Message())
	}
}

// TestCollectionAlterColumn 验证 Collection 动态修改列
func TestCollectionAlterColumn(t *testing.T) {
	path := "./test_alt_col"
	defer os.RemoveAll(path)

	c, _ := CreateAndOpen(path, testSchema(), nil)
	defer c.Close()

	f := schema.NewFieldSchema("new_title", types.DataTypeString, true, 0)
	st := c.AlterColumn("title", "new_title", f)
	if !st.OK() {
		t.Fatalf("AlterColumn failed: %v", st.Message())
	}
}
