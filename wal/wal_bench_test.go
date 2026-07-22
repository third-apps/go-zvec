package wal

import (
	"path/filepath"
	"testing"

	"github.com/third-apps/go-zvec/doc"
)

// BenchmarkWALAppendInsert 基准测试 WAL 插入操作追加性能
func BenchmarkWALAppendInsert(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.log")
	w, _ := Open(path)
	defer w.Close()

	d := doc.NewDoc("doc")
	d.SetVector("vec", doc.VectorValue{Float32s: []float32{0.1, 0.2, 0.3, 0.4}})
	d.SetStringField("name", "benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.AppendInsert("doc", d)
	}
}

// BenchmarkWALAppendDeletes 基准测试 WAL 批量删除追加性能
func BenchmarkWALAppendDeletes(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench_del.log")
	w, _ := Open(path)
	defer w.Close()

	ids := []string{"doc1", "doc2", "doc3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.AppendDeletes(ids)
	}
}

// BenchmarkWALReplay 基准测试 WAL 回放性能
func BenchmarkWALReplay(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench_replay.log")
	w, _ := Open(path)

	d := doc.NewDoc("doc")
	d.SetVector("vec", doc.VectorValue{Float32s: []float32{0.1, 0.2, 0.3, 0.4}})
	for i := 0; i < 1000; i++ {
		w.AppendInsert("doc", d)
	}
	w.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w2, _ := Open(path)
		w2.Replay(func(entry LogEntry) error {
			return nil
		})
		w2.Close()
	}
}
