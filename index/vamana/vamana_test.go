package vamana

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

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

func TestVamanaIndexEmpty(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeL2, 4, 20, 1.2, false)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

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

func TestVamanaDimension(t *testing.T) {
	idx := NewVamanaIndex(6, types.MetricTypeL2, 4, 20, 1.2, false)
	if idx.Dimension() != 6 {
		t.Fatalf("expected dimension 6, got %d", idx.Dimension())
	}
}

func TestVamanaSingleElement(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeCosine, 4, 20, 1.2, false)
	idx.Add([]float32{0.5, 0.5}, "a")
	results := idx.Search([]float32{0.5, 0.5}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestVamanaDeleteNonExistent(t *testing.T) {
	idx := NewVamanaIndex(2, types.MetricTypeL2, 4, 20, 1.2, false)
	idx.Add([]float32{1, 0}, "a")
	if idx.Delete("nonexistent") {
		t.Fatal("expected false for non-existent PK")
	}
}

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
