package hnsw

import (
	"math/rand"
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// BenchmarkHNSWSearch_1k_128 基准测试 1k 条 128 维向量 HNSW 索引搜索性能
func BenchmarkHNSWSearch_1k_128(b *testing.B) {
	idx := NewHNSWIndex(128, types.MetricTypeL2, 16, 200)
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

// BenchmarkHNSWSearch_10k_128 基准测试 10k 条 128 维向量 HNSW 索引搜索性能
func BenchmarkHNSWSearch_10k_128(b *testing.B) {
	idx := NewHNSWIndex(128, types.MetricTypeL2, 16, 200)
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
