package hnsw

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

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

func TestHNSWIndexEmpty(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

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

func TestHNSWSetEF(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.SetEF(100)
	idx.Add([]float32{1, 0}, "a")
	results := idx.Search([]float32{1, 0}, 1)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
}

func TestHNSWSetEFInvalid(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.SetEF(0)
}

func TestHNSWIndexDimension(t *testing.T) {
	idx := NewHNSWIndex(7, types.MetricTypeL2, 16, 200)
	if idx.Dimension() != 7 {
		t.Fatalf("expected dimension 7, got %d", idx.Dimension())
	}
}

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

func TestHNSWDeleteNonExistent(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeL2, 16, 200)
	idx.Add([]float32{1, 0}, "a")
	if idx.Delete("nonexistent") {
		t.Fatal("expected false for non-existent PK")
	}
}

func TestHNSWSingleElement(t *testing.T) {
	idx := NewHNSWIndex(2, types.MetricTypeCosine, 16, 200)
	idx.Add([]float32{0.5, 0.5}, "a")
	results := idx.Search([]float32{0.5, 0.5}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
