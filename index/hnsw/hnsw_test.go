package hnsw

import (
	"bytes"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestHNSWIndexBasic 验证 HNSW 图索引基本添加和搜索功能
func TestHNSWIndexBasic(t *testing.T) {
	idx := NewHNSWIndex(4, types.MetricTypeCosine, 16, 200)

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

// TestHNSWIndexEmpty 验证空 HNSW 索引搜索返回 nil
func TestHNSWIndexEmpty(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestHNSWIndexDelete 验证 HNSW 索引删除文档功能
func TestHNSWIndexDelete(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1, got %d", idx.Size())
	}
}

// TestHNSWIndexSearchWithFilter 验证 HNSW 索引带过滤条件搜索功能
func TestHNSWIndexSearchWithFilter(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")
	idx.Add([]float32{1, 1}, "doc_3")

	results := idx.SearchWithFilter([]float32{1, 0}, 5, func(pk string) bool {
		return pk != "doc_1"
	})
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	for _, r := range results {
		if r.PK == "doc_1" {
			t.Fatal("filter should have excluded doc_1")
		}
	}
}

// TestHNSWIndexLarge 验证 HNSW 索引大规模数据搜索功能
func TestHNSWIndexLarge(t *testing.T) {
	idx := NewHNSWIndex(8, types.MetricTypeCosine, 16, 200)

	for i := 0; i < 100; i++ {
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

	results := idx.Search(q, 10)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
}

// TestHNSWSetEF 验证 HNSW 索引设置搜索宽度参数 EF
func TestHNSWSetEF(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.SetEF(100)
	idx.Add([]float32{1, 0}, "a")
	results := idx.Search([]float32{1, 0}, 1)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
}

// TestHNSWSetEFInvalid 验证 HNSW 索引设置无效 EF 值不崩溃
func TestHNSWSetEFInvalid(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.SetEF(0)
}

// TestHNSWIndexDimension 验证 HNSW 索引维度查询
func TestHNSWIndexDimension(t *testing.T) {
	idx := NewHNSWIndex(7, types.MetricTypeL2, 16, 200)
	if idx.Dimension() != 7 {
		t.Fatalf("expected dimension 7, got %d", idx.Dimension())
	}
}

// TestHNSWDeleteMultiLevel 验证 HNSW 索引删除多层节点后搜索正确性
func TestHNSWDeleteMultiLevel(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Add([]float32{1, 1}, "c")
	idx.Add([]float32{0, 0}, "d")

	idx.Delete("b")
	if idx.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx.Size())
	}

	results := idx.Search([]float32{1, 0}, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.PK == "b" {
			t.Fatal("deleted doc should not appear")
		}
	}
}

// TestHNSWDeleteAll 验证 HNSW 索引删除全部文档后搜索为空
func TestHNSWDeleteAll(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
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

// TestHNSWDeleteNonExistent 验证 HNSW 索引删除不存在的 PK 返回 false
func TestHNSWDeleteNonExistent(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.Add([]float32{1, 0}, "a")
	if idx.Delete("nonexistent") {
		t.Fatal("expected false for non-existent PK")
	}
}

// TestHNSWSingleElement 验证 HNSW 索引仅含单条文档时的搜索
func TestHNSWSingleElement(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{0.5, 0.5}, "a")
	results := idx.Search([]float32{0.5, 0.5}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

// TestHNSWIndexSaveLoad 验证 HNSW 索引序列化保存与反序列化加载
func TestHNSWIndexSaveLoad(t *testing.T) {
	idx := NewHNSWIndex(4, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Delete("doc_2")

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatal(err)
	}

	idx2 := NewHNSWIndex(4, types.MetricTypeCosine, 16, 200)
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
