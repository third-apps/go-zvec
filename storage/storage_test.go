package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMemoryStorageWriteAndRead 验证内存存储写入和读取数据
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

// TestMemoryStorageReadOutOfBounds 验证内存存储越界读取返回错误
func TestMemoryStorageReadOutOfBounds(t *testing.T) {
	s := NewMemoryStorage()
	buf := make([]byte, 5)
	_, err := s.Read(0, buf)
	if err == nil {
		t.Fatal("expected error for read on empty storage")
	}
}

// TestMemoryStorageWriteGrow 验证内存存储偏移写入自动扩展
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

// TestMemoryStorageWriteOverwrite 验证内存存储覆盖写入
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

// TestMemoryStorageSyncAndClose 验证内存存储 Sync 和 Close 操作
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

// TestMemoryStorageMultipleWrites 验证内存存储多次连续写入数据拼接
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

// TestFileStorageCreateNew 验证文件存储创建新文件并写入
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

// TestFileStorageReadWrite 验证文件存储写入后只读模式读取
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

// TestFileStorageAppendExisting 验证文件存储追加写入已有文件
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

// TestFileStorageCreateDir 验证文件存储自动创建不存在的目录
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

// TestFileStorageSync 验证文件存储 Sync 刷盘操作
func TestFileStorageSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.bin")
	fs, _ := OpenFileStorage(path, StorageOptions{CreateNew: true})
	defer fs.Close()
	if err := fs.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}
