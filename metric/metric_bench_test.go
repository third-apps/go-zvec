package metric

import (
	"math/rand"
	"testing"
)

func randVec(dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rand.Float32()
	}
	return v
}

// BenchmarkL2Squared_128 基准测试 128 维 L2 平方距离计算性能
func BenchmarkL2Squared_128(b *testing.B) {
	a, v := randVec(128), randVec(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		L2Squared(a, v)
	}
}

// BenchmarkL2Squared_256 基准测试 256 维 L2 平方距离计算性能
func BenchmarkL2Squared_256(b *testing.B) {
	a, v := randVec(256), randVec(256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		L2Squared(a, v)
	}
}

// BenchmarkL2Squared_512 基准测试 512 维 L2 平方距离计算性能
func BenchmarkL2Squared_512(b *testing.B) {
	a, v := randVec(512), randVec(512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		L2Squared(a, v)
	}
}

// BenchmarkL2Squared_960 基准测试 960 维 L2 平方距离计算性能
func BenchmarkL2Squared_960(b *testing.B) {
	a, v := randVec(960), randVec(960)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		L2Squared(a, v)
	}
}

// BenchmarkInnerProduct_128 基准测试 128 维内积距离计算性能
func BenchmarkInnerProduct_128(b *testing.B) {
	a, v := randVec(128), randVec(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InnerProduct(a, v)
	}
}

// BenchmarkInnerProduct_256 基准测试 256 维内积距离计算性能
func BenchmarkInnerProduct_256(b *testing.B) {
	a, v := randVec(256), randVec(256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InnerProduct(a, v)
	}
}

// BenchmarkInnerProduct_512 基准测试 512 维内积距离计算性能
func BenchmarkInnerProduct_512(b *testing.B) {
	a, v := randVec(512), randVec(512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InnerProduct(a, v)
	}
}

// BenchmarkInnerProduct_960 基准测试 960 维内积距离计算性能
func BenchmarkInnerProduct_960(b *testing.B) {
	a, v := randVec(960), randVec(960)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InnerProduct(a, v)
	}
}

// BenchmarkCosineDistance_128 基准测试 128 维余弦距离计算性能
func BenchmarkCosineDistance_128(b *testing.B) {
	a, v := randVec(128), randVec(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineDistance(a, v)
	}
}

// BenchmarkCosineDistance_256 基准测试 256 维余弦距离计算性能
func BenchmarkCosineDistance_256(b *testing.B) {
	a, v := randVec(256), randVec(256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineDistance(a, v)
	}
}

// BenchmarkCosineDistance_512 基准测试 512 维余弦距离计算性能
func BenchmarkCosineDistance_512(b *testing.B) {
	a, v := randVec(512), randVec(512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineDistance(a, v)
	}
}

// BenchmarkNormalize_128 基准测试 128 维向量归一化性能
func BenchmarkNormalize_128(b *testing.B) {
	v := randVec(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Normalize(v)
	}
}

// BenchmarkNormalize_512 基准测试 512 维向量归一化性能
func BenchmarkNormalize_512(b *testing.B) {
	v := randVec(512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Normalize(v)
	}
}

// BenchmarkNormalizeInPlace_128 基准测试 128 维原地归一化性能
func BenchmarkNormalizeInPlace_128(b *testing.B) {
	v := randVec(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp := make([]float32, len(v))
		copy(tmp, v)
		NormalizeInPlace(tmp)
	}
}

// BenchmarkNormalizeInPlace_512 基准测试 512 维原地归一化性能
func BenchmarkNormalizeInPlace_512(b *testing.B) {
	v := randVec(512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp := make([]float32, len(v))
		copy(tmp, v)
		NormalizeInPlace(tmp)
	}
}
