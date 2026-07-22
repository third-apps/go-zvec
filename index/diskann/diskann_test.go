package diskann

import (
	"bytes"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestDiskAnnIndexBasic 验证 DiskAnn 磁盘索引基本添加和搜索功能
func TestDiskAnnIndexBasic(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)

	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")

	if idx.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx.Size())
	}

	results := idx.Search([]float32{0.9, 0.8, 0.7, 0.6}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].PK != "doc_3" {
		t.Fatalf("expected doc_3 as top result, got %s", results[0].PK)
	}
}

// TestDiskAnnIndexEmpty 验证空 DiskAnn 索引搜索返回 nil
func TestDiskAnnIndexEmpty(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	results := idx.Search([]float32{1, 2, 3, 4}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestDiskAnnIndexDelete 验证 DiskAnn 索引删除文档功能
func TestDiskAnnIndexDelete(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{1, 0, 0, 0}, "doc_1")
	idx.Add([]float32{0, 1, 0, 0}, "doc_2")

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

// TestDiskAnnIndexSearchWithFilter 验证 DiskAnn 索引带过滤条件搜索功能
func TestDiskAnnIndexSearchWithFilter(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{1, 0, 0, 0}, "doc_1")
	idx.Add([]float32{0, 1, 0, 0}, "doc_2")
	idx.Add([]float32{0, 0, 1, 0}, "doc_3")

	results := idx.SearchWithFilter([]float32{1, 0, 0, 0}, 5, func(pk string) bool {
		return pk != "doc_1"
	})
	for _, r := range results {
		if r.PK == "doc_1" {
			t.Fatal("filter should have excluded doc_1")
		}
	}
}

// TestDiskAnnIndexLarge 验证 DiskAnn 索引大规模数据搜索功能
func TestDiskAnnIndexLarge(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 64, 1.2, false)
	for i := 0; i < 100; i++ {
		v := []float32{float32(i) * 0.01, float32(i) * 0.02, float32(i) * 0.03, float32(i) * 0.04}
		idx.Add(v, "doc_large")
	}
	if idx.Size() != 100 {
		t.Fatalf("expected size 100, got %d", idx.Size())
	}
	results := idx.Search([]float32{0.5, 1.0, 1.5, 2.0}, 10)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
}

// TestDiskAnnIndexDimension 验证 DiskAnn 索引维度查询
func TestDiskAnnIndexDimension(t *testing.T) {
	idx := NewDiskAnnIndex(128, types.MetricTypeL2, 16, 32, 1.2, false)
	if idx.Dimension() != 128 {
		t.Fatalf("expected dimension 128, got %d", idx.Dimension())
	}
}

// TestDiskAnnIndexSaveLoad 验证 DiskAnn 索引序列化保存与反序列化加载
func TestDiskAnnIndexSaveLoad(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Delete("doc_2")

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatal(err)
	}

	idx2 := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	if err := idx2.Load(&buf); err != nil {
		t.Fatal(err)
	}

	if idx2.Size() != 2 {
		t.Fatalf("expected size 2, got %d", idx2.Size())
	}
}

// TestDiskAnnIndexCosine 验证 DiskAnn 索引余弦度量搜索正确性
func TestDiskAnnIndexCosine(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeCosine, 16, 32, 1.2, false)
	idx.Add([]float32{1, 0, 0, 0}, "doc_1")
	idx.Add([]float32{0, 1, 0, 0}, "doc_2")

	results := idx.Search([]float32{1, 0, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PK != "doc_1" {
		t.Fatalf("expected doc_1 as top result for cosine, got %s", results[0].PK)
	}
}

// TestDiskAnnIndexDeleteAll 验证 DiskAnn 索引删除全部文档后搜索返回 nil
func TestDiskAnnIndexDeleteAll(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{1, 0, 0, 0}, "doc_1")
	idx.Add([]float32{0, 1, 0, 0}, "doc_2")

	idx.Delete("doc_1")
	idx.Delete("doc_2")

	if idx.Size() != 0 {
		t.Fatalf("expected size 0 after deleting all, got %d", idx.Size())
	}

	results := idx.Search([]float32{1, 0, 0, 0}, 5)
	if results != nil {
		t.Fatal("expected nil results after deleting all")
	}
}

// TestDiskAnnIndexSingleElement 验证 DiskAnn 索引仅含单条文档时的搜索
func TestDiskAnnIndexSingleElement(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{1, 2, 3, 4}, "doc_1")

	results := idx.Search([]float32{1, 2, 3, 4}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].PK != "doc_1" {
		t.Fatalf("expected doc_1, got %s", results[0].PK)
	}
}

// TestDiskAnnSaturateGraph 验证 DiskAnn 索引图饱和模式搜索功能
func TestDiskAnnSaturateGraph(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, true)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")

	results := idx.Search([]float32{0.9, 0.8, 0.7, 0.6}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results with saturate graph, got %d", len(results))
	}
}

// TestDiskAnnMemoryBytes 验证 DiskAnn 索引内存占用统计非零
func TestDiskAnnMemoryBytes(t *testing.T) {
	idx := NewDiskAnnIndex(4, types.MetricTypeL2, 16, 32, 1.2, false)
	idx.Add([]float32{1, 0, 0, 0}, "doc_1")
	idx.Add([]float32{0, 1, 0, 0}, "doc_2")

	bytes := idx.MemoryBytes()
	if bytes == 0 {
		t.Fatal("expected non-zero memory usage")
	}
}
