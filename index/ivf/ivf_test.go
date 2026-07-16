package ivf

import (
	"fmt"
	"testing"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

func TestIVFIndexBasic(t *testing.T) {
	idx := NewIVFIndex(4, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Add([]float32{0.8, 0.9, 0.6, 0.7}, "doc_4")

	results := idx.Search([]float32{0.4, 0.3, 0.3, 0.1}, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Score <= 0 {
		t.Fatalf("expected positive score, got %f", results[0].Score)
	}
	_ = results[0].PK
}

func TestIVFIndexEmpty(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 10)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

func TestIVFIndexDelete(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1, got %d", idx.Size())
	}
}

func TestIVFIndexWithFilter(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 3, 10)
	for i := 0; i < 30; i++ {
		v := []float32{float32(i) / 30, float32(30-i) / 30}
		idx.Add(v, fmt.Sprintf("doc_%d", i))
	}

	results := idx.SearchWithFilter([]float32{1, 0}, 5, func(pk string) bool {
		return pk != "doc_0"
	})
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.PK == "doc_0" {
			t.Fatal("filter should have excluded doc_0")
		}
	}
}

func TestIVFIndexLarge(t *testing.T) {
	idx := NewIVFIndex(8, types.MetricTypeCosine, 3, 10)
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

func TestIVFExplicitTrain(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Add([]float32{1, 1}, "c")
	idx.Train()

	if !idx.trained {
		t.Fatal("expected trained after Train()")
	}
	if len(idx.centroids) != 2 {
		t.Fatalf("expected 2 centroids, got %d", len(idx.centroids))
	}
}

func TestIVFTrainAfterAdd(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 5)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Train()

	docID := idx.Add([]float32{1, 1}, "c")
	if idx.assignments[docID] < 0 {
		t.Fatal("expected assignment >= 0 after train")
	}
}

func TestIVFKMeansPPEdgeCases(t *testing.T) {
	data := [][]float32{{1, 2, 3}}
	centroids := kmeansPP(data, 1, metric.CosineDistance, 5)
	if len(centroids) != 1 {
		t.Fatalf("expected 1 centroid for 1 data point, got %d", len(centroids))
	}
}

func TestIVFKMeansPPMoreCentroids(t *testing.T) {
	data := [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}}
	centroids := kmeansPP(data, 4, metric.CosineDistance, 10)
	if len(centroids) != 4 {
		t.Fatalf("expected 4 centroids, got %d", len(centroids))
	}
}

func TestIVFEmptyData(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 5)
	idx.Train()
	if idx.trained {
		t.Fatal("expected not trained with no data")
	}
}

func TestIVFDeleteUpdatesInvertedIndex(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Add([]float32{1, 1}, "c")
	idx.Train()

	idx.Delete("a")
	for _, inv := range idx.inverted {
		for _, docIdx := range inv {
			if docIdx >= len(idx.docs) {
				t.Fatalf("inverted index %d out of range (len=%d)", docIdx, len(idx.docs))
			}
		}
	}
}

func TestIVFIndexDimension(t *testing.T) {
	idx := NewIVFIndex(4, types.MetricTypeL2, 2, 5)
	if idx.Dimension() != 4 {
		t.Fatalf("expected dimension 4, got %d", idx.Dimension())
	}
}

func TestIVFIndexCosineNormalize(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 1, 5)
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
