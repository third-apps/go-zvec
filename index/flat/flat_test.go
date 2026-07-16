package flat

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

func TestFlatIndexBasic(t *testing.T) {
	idx := NewFlatIndex(4, types.MetricTypeCosine)

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

	if results[0].PK != "doc_3" {
		t.Fatalf("expected doc_3 as top result (closest to query), got %s (score=%.4f)", results[0].PK, results[0].Score)
	}
}

func TestFlatIndexEmpty(t *testing.T) {
	idx := NewFlatIndex(2, types.MetricTypeL2)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

func TestFlatIndexDelete(t *testing.T) {
	idx := NewFlatIndex(2, types.MetricTypeCosine)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

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

func TestFlatIndexSearchWithFilter(t *testing.T) {
	idx := NewFlatIndex(2, types.MetricTypeCosine)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")
	idx.Add([]float32{1, 1}, "doc_3")

	results := idx.SearchWithFilter([]float32{1, 0}, 5, func(pk string) bool {
		return pk != "doc_1"
	})
	if len(results) != 2 {
		t.Fatalf("expected 2 filtered results, got %d", len(results))
	}
	for _, r := range results {
		if r.PK == "doc_1" {
			t.Fatal("filter should have excluded doc_1")
		}
	}
}

func TestFlatIndexGetDocID(t *testing.T) {
	idx := NewFlatIndex(2, types.MetricTypeCosine)
	idx.Add([]float32{1, 0}, "doc_1")
	id, found := idx.GetDocID("doc_1")
	if !found || id != 0 {
		t.Fatalf("expected doc_1 at id 0")
	}
	_, found = idx.GetDocID("nonexistent")
	if found {
		t.Fatal("expected not found")
	}
}
