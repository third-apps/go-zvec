package wal

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/third-apps/go-zvec/doc"
)

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

func TestWALAppendUpsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	d := doc.NewDoc("doc1")
	if err := w.AppendUpsert("doc1", d); err != nil {
		t.Fatalf("AppendUpsert failed: %v", err)
	}
}

func TestWALAppendUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	d := doc.NewDoc("doc1")
	if err := w.AppendUpdate("doc1", d); err != nil {
		t.Fatalf("AppendUpdate failed: %v", err)
	}
}

func TestWALAppendDeletes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	if err := w.AppendDeletes([]string{"doc1", "doc2"}); err != nil {
		t.Fatalf("AppendDeletes failed: %v", err)
	}
}

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

func TestWALSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	defer w.Close()

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}

func TestWALClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	w, _ := Open(path)
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

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
