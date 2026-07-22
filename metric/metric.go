package metric

import (
	"math"

	"github.com/third-apps/go-zvec/types"
	"golang.org/x/sys/cpu"
)

type DistanceFunc func(a, b []float32) float32

var (
	hasAVX2  = cpu.X86.HasAVX2
	hasSSE41 = cpu.X86.HasSSE41
	hasNEON  = cpu.ARM64.HasASIMD

	l2Impl func(a, b []float32) float32 = l2Generic
	ipImpl func(a, b []float32) float32 = ipGeneric
)

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
	return l2Impl(a, b)
}

func InnerProduct(a, b []float32) float32 {
	result := 1.0 - ipImpl(a, b)
	if math.IsNaN(float64(result)) || math.IsInf(float64(result), 0) {
		return float32(math.MaxFloat32)
	}
	return result
}

func l2Generic(a, b []float32) float32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n >= 64 && (hasAVX2 || hasNEON) {
		return l2Unrolled8x(a, b, n)
	}
	var sum float32
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
	if math.IsNaN(float64(sum)) || math.IsInf(float64(sum), 0) {
		return float32(math.MaxFloat32)
	}
	return sum
}

func l2Unrolled8x(a, b []float32, n int) float32 {
	var s0, s1, s2, s3, s4, s5, s6, s7 float32
	i := 0
	for ; i+7 < n; i += 8 {
		d0 := a[i] - b[i]
		d1 := a[i+1] - b[i+1]
		d2 := a[i+2] - b[i+2]
		d3 := a[i+3] - b[i+3]
		d4 := a[i+4] - b[i+4]
		d5 := a[i+5] - b[i+5]
		d6 := a[i+6] - b[i+6]
		d7 := a[i+7] - b[i+7]
		s0 += d0 * d0
		s1 += d1 * d1
		s2 += d2 * d2
		s3 += d3 * d3
		s4 += d4 * d4
		s5 += d5 * d5
		s6 += d6 * d6
		s7 += d7 * d7
	}
	sum := s0 + s1 + s2 + s3 + s4 + s5 + s6 + s7
	for ; i < n; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	if math.IsNaN(float64(sum)) || math.IsInf(float64(sum), 0) {
		return float32(math.MaxFloat32)
	}
	return sum
}

func L2(a, b []float32) float32 {
	return float32(math.Sqrt(float64(L2Squared(a, b))))
}

func ipGeneric(a, b []float32) float32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n >= 64 && (hasAVX2 || hasNEON) {
		return ipUnrolled8x(a, b, n)
	}
	var sum float32
	i := 0
	for ; i+3 < n; i += 4 {
		sum += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
	}
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func ipUnrolled8x(a, b []float32, n int) float32 {
	var s0, s1, s2, s3, s4, s5, s6, s7 float32
	i := 0
	for ; i+7 < n; i += 8 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
		s4 += a[i+4] * b[i+4]
		s5 += a[i+5] * b[i+5]
		s6 += a[i+6] * b[i+6]
		s7 += a[i+7] * b[i+7]
	}
	sum := s0 + s1 + s2 + s3 + s4 + s5 + s6 + s7
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func CosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for ; i+3 < n; i += 4 {
		dot += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
		normA += a[i]*a[i] + a[i+1]*a[i+1] + a[i+2]*a[i+2] + a[i+3]*a[i+3]
		normB += b[i]*b[i] + b[i+1]*b[i+1] + b[i+2]*b[i+2] + b[i+3]*b[i+3]
	}
	for ; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	result := 1.0 - dot/(float32(math.Sqrt(float64(normA)))*float32(math.Sqrt(float64(normB))))
	if math.IsNaN(float64(result)) || math.IsInf(float64(result), 0) {
		return 1.0
	}
	return result
}

func CosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for ; i+3 < n; i += 4 {
		dot += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
		normA += a[i]*a[i] + a[i+1]*a[i+1] + a[i+2]*a[i+2] + a[i+3]*a[i+3]
		normB += b[i]*b[i] + b[i+1]*b[i+1] + b[i+2]*b[i+2] + b[i+3]*b[i+3]
	}
	for ; i < n; i++ {
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
	for i := 0; i+3 < len(v); i += 4 {
		norm += v[i]*v[i] + v[i+1]*v[i+1] + v[i+2]*v[i+2] + v[i+3]*v[i+3]
	}
	for i := len(v) &^ 3; i < len(v); i++ {
		norm += v[i] * v[i]
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm == 0 {
		return v
	}
	result := make([]float32, len(v))
	for i := range v {
		result[i] = v[i] / norm
	}
	return result
}

func NormalizeInPlace(v []float32) {
	var norm float32
	for i := 0; i+3 < len(v); i += 4 {
		norm += v[i]*v[i] + v[i+1]*v[i+1] + v[i+2]*v[i+2] + v[i+3]*v[i+3]
	}
	for i := len(v) &^ 3; i < len(v); i++ {
		norm += v[i] * v[i]
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm == 0 {
		return
	}
	invNorm := 1.0 / norm
	for i := range v {
		v[i] *= invNorm
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
