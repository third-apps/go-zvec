package vamana

import (
	"bytes"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestVamanaIndexBasic 验证 Vamana 图索引基本添加和搜索功能
func TestVamanaIndexBasic(t *testing.T) {
	idx := NewVamanaIndex(4, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")

	if idx.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx.Size())
	}

	results := idx.Search([]float32{0.4, 0.3, 0.3, 0.1}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestVamanaIndexEmpty 验证空 Vamana 索引搜索返回 nil
func TestVamanaIndexEmpty(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeL2, 4, 20, 1.2, false)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestVamanaIndexDelete 验证 Vamana 索引删除文档功能
func TestVamanaIndexDelete(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1, got %d", idx.Size())
	}
}

// TestVamanaIndexWithFilter 验证 Vamana 索引带过滤条件搜索功能
func TestVamanaIndexWithFilter(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")
	idx.Add([]float32{1, 1}, "doc_3")

	results := idx.SearchWithFilter([]float32{1, 0}, 5, func(pk string) bool {
		return pk != "doc_1"
	})
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.PK == "doc_1" {
			t.Fatal("filter should have excluded doc_1")
		}
	}
}

// TestVamanaIndexLarge 验证 Vamana 索引大规模数据搜索功能
func TestVamanaIndexLarge(t *testing.T) {
	idx := NewVamanaIndex(8, types.MetricTypeCosine, 6, 30, 1.2, false)
	for i := 0; i < 50; i++ {
		v := make([]float32, 8)
		for j := range v {
			v[j] = float32(i*10 + j)
		}
		idx.Add(v, "")
	}

	q := make([]float32, 8)
	for j := range q {
		q[j] = float32(500 + j)
	}

	results := idx.Search(q, 5)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

// TestVamanaSaturateGraph 验证 Vamana 索引图饱和模式搜索功能
func TestVamanaSaturateGraph(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 3, 10, 1.2, true)
	for i := 0; i < 10; i++ {
		v := []float32{float32(i) / 10, float32(10-i) / 10}
		idx.Add(v, "")
	}

	results := idx.Search([]float32{0.5, 0.5}, 5)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

// TestVamanaDimension 验证 Vamana 索引维度查询
func TestVamanaDimension(t *testing.T) {
	idx := NewVamanaIndex(6, types.MetricTypeL2, 4, 20, 1.2, false)
	if idx.Dimension() != 6 {
		t.Fatalf("expected dimension 6, got %d", idx.Dimension())
	}
}

// TestVamanaSingleElement 验证 Vamana 索引仅含单条文档时的搜索
func TestVamanaSingleElement(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{0.5, 0.5}, "a")
	results := idx.Search([]float32{0.5, 0.5}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// TestVamanaDeleteNonExistent 验证 Vamana 索引删除不存在的 PK 返回 false
func TestVamanaDeleteNonExistent(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeL2, 4, 20, 1.2, false)
	idx.Add([]float32{1, 0}, "a")
	if idx.Delete("nonexistent") {
		t.Fatal("expected false for non-existent PK")
	}
}

// TestVamanaDeleteAll 验证 Vamana 索引删除全部文档后搜索为空
func TestVamanaDeleteAll(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{1, 0}, "a")
	idx.Delete("a")
	if idx.Size() != 0 {
		t.Fatal("expected size 0")
	}
	results := idx.Search([]float32{1, 0}, 5)
	if len(results) != 0 {
		t.Fatal("expected empty results for tombstone-deleted index")
	}
}

// TestVamanaL2Metric 验证 Vamana 索引 L2 度量搜索正确性
func TestVamanaL2Metric(t *testing.T) {
	idx := NewVamanaIndex(3, types.MetricTypeL2, 4, 20, 1.2, false)
	idx.Add([]float32{3, 4, 0}, "a")
	idx.Add([]float32{0, 0, 0}, "b")
	results := idx.Search([]float32{0, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PK != "b" {
		t.Fatalf("expected 'b' (zero) as closest to zero query, got %s", results[0].PK)
	}
}

// TestVamanaCosineNormalize 验证 Vamana 索引余弦度量下向量归一化搜索正确性
func TestVamanaCosineNormalize(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{3, 4}, "a")
	idx.Add([]float32{4, 3}, "b")
	results := idx.Search([]float32{3, 4}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PK != "a" {
		t.Fatalf("expected 'a' as top result, got %s", results[0].PK)
	}
}

// TestVamanaIndexSaveLoad 验证 Vamana 索引序列化保存与反序列化加载
func TestVamanaIndexSaveLoad(t *testing.T) {
	idx := NewVamanaIndex(4, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Delete("doc_2")

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatal(err)
	}

	idx2 := NewVamanaIndex(4, types.MetricTypeCosine, 4, 20, 1.2, false)
	if err := idx2.Load(&buf); err != nil {
		t.Fatal(err)
	}

	if idx2.Size() != 2 {
		t.Fatalf("expected size 2, got %d", idx2.Size())
	}

	results := idx2.Search([]float32{0.4, 0.3, 0.3, 0.1}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
