package metric

import (
	"math"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

func TestL2Squared(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{3, 4, 0}
	d := L2Squared(a, b)
	if math.Abs(float64(d-25)) > 0.0001 {
		t.Fatalf("expected 25, got %f", d)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)) > 0.0001 {
		t.Fatalf("expected 0, got %f", sim)
	}
}

func TestNormalize(t *testing.T) {
	v := []float32{3, 4}
	n := Normalize(v)
	expected := float32(1.0)
	var norm float32
	for _, x := range n {
		norm += x * x
	}
	if math.Abs(float64(norm-expected)) > 0.0001 {
		t.Fatalf("expected norm 1, got %f", norm)
	}
}

func TestGetDistanceFunc(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}

	l2 := GetDistanceFunc(types.MetricTypeL2)
	l2Dist := l2(a, b)
	expected := float32((1-4)*(1-4) + (2-5)*(2-5) + (3-6)*(3-6))
	if l2Dist != expected {
		t.Fatalf("L2 distance mismatch")
	}
}

func TestL2(t *testing.T) {
	a := []float32{3, 4}
	b := []float32{0, 0}
	d := L2(a, b)
	if math.Abs(float64(d-5.0)) > 0.0001 {
		t.Fatalf("expected 5, got %f", d)
	}
}

func TestInnerProduct(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	d := InnerProduct(a, b)
	if math.Abs(float64(d-1.0)) > 0.0001 {
		t.Fatalf("expected 1 (1 - 0), got %f", d)
	}

	a2 := []float32{2, 3}
	b2 := []float32{4, 5}
	d2 := InnerProduct(a2, b2)
	expected := 1.0 - float32(2*4+3*5)
	if math.Abs(float64(d2-expected)) > 0.0001 {
		t.Fatalf("expected %f, got %f", expected, d2)
	}
}

func TestCosineDistance(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	d := CosineDistance(a, b)
	if math.Abs(float64(d-1.0)) > 0.0001 {
		t.Fatalf("expected 1 for orthogonal, got %f", d)
	}

	c := []float32{2, 0, 0}
	d2 := CosineDistance(a, c)
	if math.Abs(float64(d2)) > 0.0001 {
		t.Fatalf("expected 0 for same direction, got %f", d2)
	}

	e := []float32{-1, 0, 0}
	d3 := CosineDistance(a, e)
	if math.Abs(float64(d3-2.0)) > 0.0001 {
		t.Fatalf("expected 2 for opposite direction, got %f", d3)
	}

	zero := []float32{0, 0, 0}
	d4 := CosineDistance(a, zero)
	if math.Abs(float64(d4-1.0)) > 0.0001 {
		t.Fatalf("expected 1 for zero vector, got %f", d4)
	}
}

func TestSparseInnerProduct(t *testing.T) {
	ia := []uint32{0, 2, 5}
	va := []float32{1.0, 2.0, 3.0}
	ib := []uint32{0, 1, 5}
	vb := []float32{4.0, 5.0, 6.0}

	d := SparseInnerProduct(ia, va, ib, vb)
	expected := 1.0 - float32(1.0*4.0+3.0*6.0)
	if math.Abs(float64(d-expected)) > 0.0001 {
		t.Fatalf("expected %f, got %f", expected, d)
	}
}

func TestSparseInnerProductNoOverlap(t *testing.T) {
	ia := []uint32{0, 1}
	va := []float32{1, 2}
	ib := []uint32{2, 3}
	vb := []float32{3, 4}

	d := SparseInnerProduct(ia, va, ib, vb)
	if math.Abs(float64(d-1.0)) > 0.0001 {
		t.Fatalf("expected 1 for no overlap, got %f", d)
	}
}

func TestNormalizeZeroVector(t *testing.T) {
	v := []float32{0, 0, 0}
	n := Normalize(v)
	var norm float32
	for _, x := range n {
		norm += x * x
	}
	if math.Abs(float64(norm)) > 0.0001 {
		t.Fatalf("expected 0 norm for zero vector, got %f", norm)
	}
}

func TestGetDistanceFuncUnknown(t *testing.T) {
	fn := GetDistanceFunc(types.MetricType(999))
	if fn == nil {
		t.Fatal("expected non-nil function")
	}
	a := []float32{1, 2}
	b := []float32{3, 4}
	result := fn(a, b)
	if result <= 0 {
		t.Fatal("expected positive distance")
	}
}

func TestCosineSameVector(t *testing.T) {
	a := []float32{0.5, 0.5, 0.5, 0.5}
	d := CosineDistance(a, a)
	if math.Abs(float64(d)) > 0.0001 {
		t.Fatalf("expected 0 for same vector, got %f", d)
	}
}

func TestCosineSimilaritySame(t *testing.T) {
	a := []float32{3, 4}
	sim := CosineSimilarity(a, a)
	if math.Abs(float64(sim-1.0)) > 0.0001 {
		t.Fatalf("expected 1 for same vector, got %f", sim)
	}
}

func TestCosineSimilarityZero(t *testing.T) {
	a := []float32{1, 0}
	zero := []float32{0, 0}
	sim := CosineSimilarity(a, zero)
	if math.Abs(float64(sim)) > 0.0001 {
		t.Fatalf("expected 0 for zero vector, got %f", sim)
	}
}
