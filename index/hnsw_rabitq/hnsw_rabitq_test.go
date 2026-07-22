package hnsw_rabitq

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestHNSWRabitqIndexBasic 验证 HNSW RaBitQ 量化索引基本添加和搜索功能
func TestHNSWRabitqIndexBasic(t *testing.T) {
	idx := NewHNSWRabitqIndex(4, types.MetricTypeCosine, 16, 200)

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

// TestHNSWRabitqIndexEmpty 验证空 HNSW RaBitQ 索引搜索返回 nil
func TestHNSWRabitqIndexEmpty(t *testing.T) {
	idx := NewHNSWRabitqIndex(2, types.MetricTypeL2, 16, 200)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestHNSWRabitqIndexDelete 验证 HNSW RaBitQ 索引删除文档功能
func TestHNSWRabitqIndexDelete(t *testing.T) {
	idx := NewHNSWRabitqIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1, got %d", idx.Size())
	}
}

// TestHNSWRabitqIndexSaveLoad 验证 HNSW RaBitQ 索引序列化保存与反序列化加载
func TestHNSWRabitqIndexSaveLoad(t *testing.T) {
	idx := NewHNSWRabitqIndex(4, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatal(err)
	}

	idx2 := NewHNSWRabitqIndex(4, types.MetricTypeCosine, 16, 200)
	if err := idx2.Load(&buf); err != nil {
		t.Fatal(err)
	}

	if idx2.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx2.Size())
	}

	results := idx2.Search([]float32{0.4, 0.3, 0.3, 0.1}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// TestHNSWRabitqIndexLarge 验证 HNSW RaBitQ 索引大规模数据搜索功能
func TestHNSWRabitqIndexLarge(t *testing.T) {
	idx := NewHNSWRabitqIndex(8, types.MetricTypeCosine, 16, 200)

	for i := 0; i < 100; i++ {
		v := make([]float32, 8)
		for j := range v {
			v[j] = float32(i*10+j) / 1000.0
		}
		idx.Add(v, fmt.Sprintf("doc_%d", i))
	}

	q := make([]float32, 8)
	for j := range q {
		q[j] = float32(500+j) / 1000.0
	}

	results := idx.Search(q, 10)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
}

// TestHNSWRabitqIndexDimension 验证 HNSW RaBitQ 索引维度查询
func TestHNSWRabitqIndexDimension(t *testing.T) {
	idx := NewHNSWRabitqIndex(7, types.MetricTypeL2, 16, 200)
	if idx.Dimension() != 7 {
		t.Fatalf("expected dimension 7, got %d", idx.Dimension())
	}
}
