package flat

import (
	"math/rand"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// BenchmarkFlatSearch_1k_128 基准测试 1k 条 128 维向量 Flat 索引搜索性能
func BenchmarkFlatSearch_1k_128(b *testing.B) {
	idx := NewFlatIndex(128, types.MetricTypeL2)
	for i := 0; i < 1000; i++ {
		v := make([]float32, 128)
		for j := range v {
			v[j] = rand.Float32()
		}
		idx.Add(v, "doc")
	}

	query := make([]float32, 128)
	for j := range query {
		query[j] = rand.Float32()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}

// BenchmarkFlatSearch_10k_128 基准测试 10k 条 128 维向量 Flat 索引搜索性能
func BenchmarkFlatSearch_10k_128(b *testing.B) {
	idx := NewFlatIndex(128, types.MetricTypeL2)
	for i := 0; i < 10000; i++ {
		v := make([]float32, 128)
		for j := range v {
			v[j] = rand.Float32()
		}
		idx.Add(v, "doc")
	}

	query := make([]float32, 128)
	for j := range query {
		query[j] = rand.Float32()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}
