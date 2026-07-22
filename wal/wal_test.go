package wal

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/third-apps/go-zvec/doc"
)

// TestWALCreateAndAppendInsert 验证 WAL 文件创建及插入操作追加
func TestWALCreateAndAppendInsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer w.Close()

	d := doc.NewDoc("doc1")
	if err := w.AppendInsert("doc1", d); err != nil {
		t.Fatalf("AppendInsert failed: %v", err)
	}
}

// TestWALAppendUpsert 验证 WAL 追加 Upsert 操作
func TestWALAppendUpsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	d := doc.NewDoc("doc1")
	if err := w.AppendUpsert("doc1", d); err != nil {
		t.Fatalf("AppendUpsert failed: %v", err)
	}
}

// TestWALAppendUpdate 验证 WAL 追加 Update 操作
func TestWALAppendUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	d := doc.NewDoc("doc1")
	if err := w.AppendUpdate("doc1", d); err != nil {
		t.Fatalf("AppendUpdate failed: %v", err)
	}
}

// TestWALAppendDeletes 验证 WAL 追加批量删除操作
func TestWALAppendDeletes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	if err := w.AppendDeletes([]string{"doc1", "doc2"}); err != nil {
		t.Fatalf("AppendDeletes failed: %v", err)
	}
}

// TestWALMultipleOps 验证 WAL 连续追加多种操作后文件非空
func TestWALMultipleOps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.AppendUpsert("doc2", doc.NewDoc("doc2"))
	w.AppendUpdate("doc1", doc.NewDoc("doc1"))
	w.AppendDeletes([]string{"doc2"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty WAL file")
	}
}

// TestWALSync 验证 WAL 手动刷盘操作
func TestWALSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}

// TestWALClose 验证 WAL 文件正常关闭
func TestWALClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestWALFileContentJSON 验证 WAL 写入文件后包含有效内容
func TestWALFileContentJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	d := doc.NewDoc("doc1")
	d.SetStringField("name", "alice")
	w.AppendInsert("doc1", d)
	w.Close()

	data, _ := os.ReadFile(path)
	content := string(data)
	if len(content) == 0 {
		t.Fatal("expected JSON content")
	}
}

// TestWALPathVariants 验证 WAL 在不同路径（含子目录）下创建文件
func TestWALPathVariants(t *testing.T) {
	tests := []string{
		filepath.Join(t.TempDir(), "wal.log"),
		filepath.Join(t.TempDir(), "sub", "wal.log"),
	}
	for _, path := range tests {
		w, err := Open(path)
		if err != nil {
			t.Fatalf("Open(%q) failed: %v", path, err)
		}
		w.AppendInsert("x", doc.NewDoc("x"))
		w.Close()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected file at %s", path)
		}
	}
}

// TestWALConcurrentAppends 验证 WAL 并发追加写入的安全性
func TestWALConcurrentAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.log")
	w, _ := Open(path)
	defer w.Close()

	var wg sync.WaitGroup
	n := 20
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			d := doc.NewDoc("")
			err := w.AppendInsert("doc", d)
			if err != nil {
				t.Errorf("concurrent append failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

// TestWALReplayInsert 验证 WAL 回放插入操作及文档字段正确性
func TestWALReplayInsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.log")
	w, _ := Open(path)

	d1 := doc.NewDoc("doc1")
	d1.SetStringField("name", "alice")
	d1.SetVector("vec", doc.VectorValue{Float32s: []float32{0.1, 0.2, 0.3}})
	w.AppendInsert("doc1", d1)

	d2 := doc.NewDoc("doc2")
	d2.SetStringField("name", "bob")
	w.AppendInsert("doc2", d2)
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	var entries []LogEntry
	err := w2.Replay(func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Op != OpInsert || entries[0].ID != "doc1" {
		t.Fatalf("expected INSERT doc1, got %s %s", entries[0].Op, entries[0].ID)
	}
	if entries[1].Op != OpInsert || entries[1].ID != "doc2" {
		t.Fatalf("expected INSERT doc2, got %s %s", entries[1].Op, entries[1].ID)
	}
	if entries[0].Doc == nil {
		t.Fatal("expected doc in first entry")
	}
	name, _ := entries[0].Doc.Field("name")
	if name.StringVal != "alice" {
		t.Fatalf("expected name=alice, got %v", name.StringVal)
	}
}

// TestWALReplayMixedOps 验证 WAL 回放混合操作（Insert/Upsert/Update/Delete）的顺序和类型
func TestWALReplayMixedOps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_mixed.log")
	w, _ := Open(path)

	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.AppendUpsert("doc2", doc.NewDoc("doc2"))
	w.AppendUpdate("doc1", doc.NewDoc("doc1"))
	w.AppendDeletes([]string{"doc2"})
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	var entries []LogEntry
	err := w2.Replay(func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Op != OpInsert {
		t.Fatalf("expected INSERT, got %s", entries[0].Op)
	}
	if entries[1].Op != OpUpsert {
		t.Fatalf("expected UPSERT, got %s", entries[1].Op)
	}
	if entries[2].Op != OpUpdate {
		t.Fatalf("expected UPDATE, got %s", entries[2].Op)
	}
	if entries[3].Op != OpDelete {
		t.Fatalf("expected DELETE, got %s", entries[3].Op)
	}
	if len(entries[3].IDs) != 1 || entries[3].IDs[0] != "doc2" {
		t.Fatalf("expected delete IDs [doc2], got %v", entries[3].IDs)
	}
}

// TestWALReplayLSN 验证 WAL 回放时 LSN 单调递增
func TestWALReplayLSN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_lsn.log")
	w, _ := Open(path)

	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.AppendInsert("doc2", doc.NewDoc("doc2"))
	w.AppendInsert("doc3", doc.NewDoc("doc3"))
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	var entries []LogEntry
	w2.Replay(func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})

	for i, e := range entries {
		if e.LSN != uint64(i+1) {
			t.Fatalf("entry %d: expected LSN %d, got %d", i, i+1, e.LSN)
		}
	}
}

// TestWALReplayWithVectors 验证 WAL 回放时稠密向量和稀疏向量的完整性
func TestWALReplayWithVectors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_vec.log")
	w, _ := Open(path)

	d := doc.NewDoc("doc1")
	d.SetVector("embedding", doc.VectorValue{Float32s: []float32{0.1, 0.2, 0.3, 0.4}})
	d.SetSparseVector("sparse", doc.SparseVectorValue{
		Indices: []uint32{0, 5, 10},
		Values:  []float32{0.5, 0.3, 0.2},
	})
	w.AppendInsert("doc1", d)
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	var entries []LogEntry
	w2.Replay(func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	vec, ok := entries[0].Doc.Vector("embedding")
	if !ok {
		t.Fatal("expected embedding vector")
	}
	if len(vec.Float32s) != 4 {
		t.Fatalf("expected 4 float32s, got %d", len(vec.Float32s))
	}
	sv, ok := entries[0].Doc.SparseVector("sparse")
	if !ok {
		t.Fatal("expected sparse vector")
	}
	if len(sv.Indices) != 3 || len(sv.Values) != 3 {
		t.Fatalf("expected 3 sparse entries, got indices=%d values=%d", len(sv.Indices), len(sv.Values))
	}
}

// TestWALReplayStopsOnError 验证 WAL 回放在回调返回错误时立即停止
func TestWALReplayStopsOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_err.log")
	w, _ := Open(path)
	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.AppendInsert("doc2", doc.NewDoc("doc2"))
	w.AppendInsert("doc3", doc.NewDoc("doc3"))
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	count := 0
	err := w2.Replay(func(entry LogEntry) error {
		count++
		if count == 2 {
			return io.EOF
		}
		return nil
	})
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries processed, got %d", count)
	}
}

// TestWALReplayEmptyFile 验证 WAL 回放空文件时无条目且不报错
func TestWALReplayEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_empty.log")
	w, _ := Open(path)
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()

	count := 0
	err := w2.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("Replay on empty file failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 entries, got %d", count)
	}
}

// TestWALReplayReadOnly 验证 WAL 只读模式回放条目
func TestWALReplayReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_ro.log")
	w, _ := Open(path)
	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly failed: %v", err)
	}

	count := 0
	ro.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	ro.Close()
	if count != 1 {
		t.Fatalf("expected 1 entry from read-only replay, got %d", count)
	}
}

// TestWALReplayCorruptEntry 验证 WAL 回放时跳过损坏条目并保留有效条目
func TestWALReplayCorruptEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_corrupt.log")
	w, _ := Open(path)
	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.Close()

	f, _ := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	f.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0xDE, 0xAD})
	f.Close()

	w2, _ := Open(path)
	defer w2.Close()

	count := 0
	w2.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if count != 1 {
		t.Fatalf("expected 1 valid entry (corrupt skipped), got %d", count)
	}
}

// TestWALTruncate 验证 WAL 截断后仅保留新追加的条目
func TestWALTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncate.log")
	w, _ := Open(path)
	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.AppendInsert("doc2", doc.NewDoc("doc2"))
	w.Close()

	w2, _ := Open(path)
	if err := w2.Truncate(); err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}
	w2.AppendInsert("doc3", doc.NewDoc("doc3"))
	w2.Close()

	w3, _ := Open(path)
	defer w3.Close()

	count := 0
	w3.Replay(func(entry LogEntry) error {
		count++
		if entry.ID != "doc3" {
			t.Fatalf("expected only doc3 after truncate, got %s", entry.ID)
		}
		return nil
	})
	if count != 1 {
		t.Fatalf("expected 1 entry after truncate, got %d", count)
	}
}

// TestWALTruncateThenReplay 验证 WAL 截断后回放为空，再追加后回放正确
func TestWALTruncateThenReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncate_replay.log")
	w, _ := Open(path)
	w.AppendInsert("doc1", doc.NewDoc("doc1"))
	w.Truncate()

	count := 0
	w.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if count != 0 {
		t.Fatalf("expected 0 entries after truncate, got %d", count)
	}

	w.AppendInsert("doc2", doc.NewDoc("doc2"))
	w.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if count != 1 {
		t.Fatalf("expected 1 entry after truncate+append, got %d", count)
	}
	w.Close()
}

// TestWALReopenAppend 验证 WAL 重新打开后可继续追加写入
func TestWALReopenAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.log")
	w1, _ := Open(path)
	w1.AppendInsert("doc1", doc.NewDoc("doc1"))
	w1.Close()

	w2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer w2.Close()
	w2.AppendInsert("doc2", doc.NewDoc("doc2"))
	w2.Close()

	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("expected data after reopen")
	}
}

// TestWALBatchSync 验证 WAL 定时批量刷盘后条目完整
func TestWALBatchSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch_sync.log")
	w, err := OpenWithSyncInterval(path, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("OpenWithSyncInterval failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		w.AppendInsert("doc", doc.NewDoc("doc"))
	}

	time.Sleep(100 * time.Millisecond)

	count := 0
	w.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if count != 100 {
		t.Fatalf("expected 100 entries after batch sync, got %d", count)
	}
	w.Close()
}

// TestWALBatchSyncClose 验证 WAL 关闭时自动刷盘确保条目完整
func TestWALBatchSyncClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch_sync_close.log")
	w, _ := OpenWithSyncInterval(path, 10*time.Second)

	for i := 0; i < 50; i++ {
		w.AppendInsert("doc", doc.NewDoc("doc"))
	}
	w.Close()

	w2, _ := Open(path)
	defer w2.Close()
	count := 0
	w2.Replay(func(entry LogEntry) error {
		count++
		return nil
	})
	if count != 50 {
		t.Fatalf("expected 50 entries after close, got %d", count)
	}
}
