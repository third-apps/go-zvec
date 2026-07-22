package flat_sparse

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestSparseIndexBasic 验证稀疏向量索引基本添加和搜索功能
func TestSparseIndexBasic(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)

	idx.Add([]uint32{0, 1, 2}, []float32{0.1, 0.2, 0.3}, "doc_1")
	idx.Add([]uint32{1, 2, 3}, []float32{0.4, 0.5, 0.6}, "doc_2")
	idx.Add([]uint32{0, 3}, []float32{0.7, 0.8}, "doc_3")

	if idx.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx.Size())
	}

	results := idx.Search([]uint32{0, 1, 2}, []float32{0.1, 0.2, 0.3}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestSparseIndexEmpty 验证空稀疏索引搜索返回 nil
func TestSparseIndexEmpty(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	results := idx.Search([]uint32{0, 1}, []float32{0.5, 0.5}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestSparseIndexDelete 验证稀疏索引删除文档功能
func TestSparseIndexDelete(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	idx.Add([]uint32{0, 1}, []float32{1.0, 0.0}, "doc_1")
	idx.Add([]uint32{0, 1}, []float32{0.0, 1.0}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1 after delete, got %d", idx.Size())
	}
	if idx.Delete("nonexistent") {
		t.Fatal("expected delete to fail for nonexistent pk")
	}
}

// TestSparseIndexSearchWithFilter 验证稀疏索引带过滤条件搜索功能
func TestSparseIndexSearchWithFilter(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	idx.Add([]uint32{0, 1}, []float32{1.0, 0.0}, "doc_1")
	idx.Add([]uint32{0, 1}, []float32{0.0, 1.0}, "doc_2")
	idx.Add([]uint32{0, 1}, []float32{0.5, 0.5}, "doc_3")

	results := idx.SearchWithFilter([]uint32{0, 1}, []float32{1.0, 0.0}, 5, func(pk string) bool {
		return pk != "doc_1"
	})
	for _, r := range results {
		if r.PK == "doc_1" {
			t.Fatal("filter should have excluded doc_1")
		}
	}
}

// TestSparseIndexDeleteAll 验证稀疏索引删除全部文档后搜索返回 nil
func TestSparseIndexDeleteAll(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	idx.Add([]uint32{0}, []float32{1.0}, "doc_1")
	idx.Add([]uint32{1}, []float32{1.0}, "doc_2")

	idx.Delete("doc_1")
	idx.Delete("doc_2")

	if idx.Size() != 0 {
		t.Fatalf("expected size 0 after deleting all, got %d", idx.Size())
	}

	results := idx.Search([]uint32{0}, []float32{1.0}, 5)
	if results != nil {
		t.Fatal("expected nil results after deleting all")
	}
}

// TestSparseIndexSingleElement 验证稀疏索引仅含单条文档时的搜索
func TestSparseIndexSingleElement(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	idx.Add([]uint32{0, 1, 2}, []float32{0.5, 0.5, 0.5}, "doc_1")

	results := idx.Search([]uint32{0, 1, 2}, []float32{0.5, 0.5, 0.5}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].PK != "doc_1" {
		t.Fatalf("expected doc_1, got %s", results[0].PK)
	}
}

// TestSparseIndexNoOverlap 验证稀疏索引无索引重叠时的搜索排序
func TestSparseIndexNoOverlap(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	idx.Add([]uint32{0, 1}, []float32{1.0, 1.0}, "doc_1")
	idx.Add([]uint32{2, 3}, []float32{1.0, 1.0}, "doc_2")

	results := idx.Search([]uint32{0, 1}, []float32{1.0, 1.0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PK != "doc_1" {
		t.Fatalf("expected doc_1 as top result (has overlap), got %s", results[0].PK)
	}
}

// TestSparseIndexClose 验证稀疏索引关闭操作
func TestSparseIndexClose(t *testing.T) {
	idx := NewSparseIndex(types.MetricTypeIP)
	if err := idx.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
