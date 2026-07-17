package metric

import (
	"math"

	"github.com/third-apps/go-zvec/types"
)

type DistanceFunc func(a, b []float32) float32

func GetDistanceFunc(metric types.MetricType) DistanceFunc {
	switch metric {
	case types.MetricTypeL2:
		return L2Squared
	case types.MetricTypeIP:
		return InnerProduct
	case types.MetricTypeCosine:
		return InnerProduct
	case types.MetricTypeMIPSL2:
		return L2Squared
	default:
		return L2Squared
	}
}

func L2Squared(a, b []float32) float32 {
	var sum float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for ; i+3 < n; i += 4 {
		d0 := a[i] - b[i]
		d1 := a[i+1] - b[i+1]
		d2 := a[i+2] - b[i+2]
		d3 := a[i+3] - b[i+3]
		sum += d0*d0 + d1*d1 + d2*d2 + d3*d3
	}
	for ; i < n; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum
}

func L2(a, b []float32) float32 {
	return float32(math.Sqrt(float64(L2Squared(a, b))))
}

func InnerProduct(a, b []float32) float32 {
	var sum float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for ; i+3 < n; i += 4 {
		sum += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
	}
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}
	return 1.0 - sum
}

func CosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	return 1.0 - dot/(float32(math.Sqrt(float64(normA)))*float32(math.Sqrt(float64(normB))))
}

func CosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func Normalize(v []float32) []float32 {
	var norm float32
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm == 0 {
		return v
	}
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
}

func NormalizeInPlace(v []float32) {
	var norm float32
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm == 0 {
		return
	}
	for i := range v {
		v[i] /= norm
	}
}

func SparseInnerProduct(indicesA []uint32, valuesA []float32, indicesB []uint32, valuesB []float32) float32 {
	var sum float32
	i, j := 0, 0
	for i < len(indicesA) && j < len(indicesB) {
		if indicesA[i] == indicesB[j] {
			sum += valuesA[i] * valuesB[j]
			i++
			j++
		} else if indicesA[i] < indicesB[j] {
			i++
		} else {
			j++
		}
	}
	return 1.0 - sum
}
