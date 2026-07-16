package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryStorageWriteAndRead(t *testing.T) {
	s := NewMemoryStorage()
	data := []byte("hello world")
	if err := s.Write(0, data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	size, err := s.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("Size = %d, want %d", size, len(data))
	}
	buf := make([]byte, 5)
	n, err := s.Read(0, buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 || string(buf[:n]) != "hello" {
		t.Fatalf("Read = %q, want %q", string(buf[:n]), "hello")
	}
}

func TestMemoryStorageReadOutOfBounds(t *testing.T) {
	s := NewMemoryStorage()
	buf := make([]byte, 5)
	_, err := s.Read(0, buf)
	if err == nil {
		t.Fatal("expected error for read on empty storage")
	}
}

func TestMemoryStorageWriteGrow(t *testing.T) {
	s := NewMemoryStorage()
	if err := s.Write(100, []byte("A")); err != nil {
		t.Fatalf("Write at offset 100 failed: %v", err)
	}
	size, _ := s.Size()
	if size != 101 {
		t.Fatalf("Size = %d, want 101", size)
	}
	buf := make([]byte, 101)
	n, _ := s.Read(0, buf)
	if n != 101 || buf[100] != 'A' {
		t.Fatal("Write grow failed: data mismatch")
	}
}

func TestMemoryStorageWriteOverwrite(t *testing.T) {
	s := NewMemoryStorage()
	s.Write(0, []byte("hello"))
	s.Write(0, []byte("HELLO"))
	buf := make([]byte, 5)
	s.Read(0, buf)
	if string(buf) != "HELLO" {
		t.Fatalf("overwrite failed: got %q", string(buf))
	}
}

func TestMemoryStorageSyncAndClose(t *testing.T) {
	s := NewMemoryStorage()
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	size, _ := s.Size()
	if size != 0 {
		t.Fatal("expected size 0 after close (data nil)")
	}
}

func TestMemoryStorageMultipleWrites(t *testing.T) {
	s := NewMemoryStorage()
	s.Write(0, []byte("aaa"))
	s.Write(3, []byte("bbb"))
	s.Write(6, []byte("ccc"))
	buf := make([]byte, 9)
	s.Read(0, buf)
	if string(buf) != "aaabbbccc" {
		t.Fatalf("multiple writes: got %q", string(buf))
	}
}

func TestFileStorageCreateNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	opts := StorageOptions{CreateNew: true}
	fs, err := OpenFileStorage(path, opts)
	if err != nil {
		t.Fatalf("OpenFileStorage failed: %v", err)
	}
	defer fs.Close()

	if err := fs.Write(0, []byte("data")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	size, err := fs.Size()
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != 4 {
		t.Fatalf("Size = %d, want 4", size)
	}
}

func TestFileStorageReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	fs, _ := OpenFileStorage(path, StorageOptions{CreateNew: true})
	fs.Write(0, []byte("hello world"))
	fs.Close()

	fs2, err := OpenFileStorage(path, StorageOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("Open read-only failed: %v", err)
	}
	defer fs2.Close()

	buf := make([]byte, 5)
	n, err := fs2.Read(0, buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 || string(buf[:n]) != "hello" {
		t.Fatalf("Read = %q, want %q", string(buf[:n]), "hello")
	}
}

func TestFileStorageAppendExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "append.bin")
	fs1, _ := OpenFileStorage(path, StorageOptions{CreateNew: true})
	fs1.Write(0, []byte("first"))
	fs1.Close()

	fs2, err := OpenFileStorage(path, StorageOptions{})
	if err != nil {
		t.Fatalf("Open existing failed: %v", err)
	}
	defer fs2.Close()

	fs2.Write(5, []byte("second"))
	size, _ := fs2.Size()
	if size != 11 {
		t.Fatalf("Size = %d, want 11", size)
	}
	buf := make([]byte, 11)
	fs2.Read(0, buf)
	if string(buf) != "firstsecond" {
		t.Fatalf("append = %q, want %q", string(buf), "firstsecond")
	}
}

func TestFileStorageCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "test.bin")
	fs, err := OpenFileStorage(path, StorageOptions{CreateNew: true})
	if err != nil {
		t.Fatalf("OpenFileStorage with new dirs failed: %v", err)
	}
	fs.Close()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected file to exist")
	}
}

func TestFileStorageSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.bin")
	fs, _ := OpenFileStorage(path, StorageOptions{CreateNew: true})
	defer fs.Close()
	if err := fs.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}
