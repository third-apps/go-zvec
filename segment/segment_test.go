package segment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/third-apps/go-zvec/doc"
)

// TestSegmentInsert 验证 Segment 插入文档后文档计数正确
func TestSegmentInsert(t *testing.T) {
	s := NewSegment(0, 100)
	d := doc.NewDoc("doc1")
	d.DocID = 1
	s.Insert(d)

	if s.DocCount() != 1 {
		t.Fatalf("expected 1, got %d", s.DocCount())
	}
}

// TestSegmentIsFull 验证 Segment 达到容量上限时 IsFull 返回 true
func TestSegmentIsFull(t *testing.T) {
	s := NewSegment(0, 2)
	s.Insert(doc.NewDoc("a"))
	s.Insert(doc.NewDoc("b"))
	if !s.IsFull() {
		t.Fatal("expected full")
	}
}

// TestSegmentDelete 验证 Segment 删除文档后文档计数为0
func TestSegmentDelete(t *testing.T) {
	s := NewSegment(0, 100)
	d := doc.NewDoc("doc1")
	d.DocID = 1
	s.Insert(d)

	if !s.Delete("doc1") {
		t.Fatal("expected delete to succeed")
	}
	if s.DocCount() != 0 {
		t.Fatalf("expected 0, got %d", s.DocCount())
	}
}

// TestSegmentGetDoc 验证 Segment 按 PK 获取文档
func TestSegmentGetDoc(t *testing.T) {
	s := NewSegment(0, 100)
	d := doc.NewDoc("doc1")
	d.DocID = 1
	s.Insert(d)

	got := s.GetDoc("doc1")
	if got == nil || got.ID != "doc1" {
		t.Fatal("expected to get doc1")
	}
}

// TestSegmentSaveLoad 验证 Segment 序列化保存与反序列化加载
func TestSegmentSaveLoad(t *testing.T) {
	s := NewSegment(0, 100)
	d1 := doc.NewDoc("doc1")
	d1.DocID = 1
	d1.SetStringField("name", "test")
	s.Insert(d1)

	dir := filepath.Join(os.TempDir(), "go-zvec-seg-test")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "seg.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}

	s2, err := LoadSegment(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.DocCount() != 1 {
		t.Fatalf("expected 1, got %d", s2.DocCount())
	}
	got := s2.GetDoc("doc1")
	if got == nil || got.ID != "doc1" {
		t.Fatal("expected to get doc1 after load")
	}
}

// TestManagerInsert 验证 Segment Manager 插入文档后文档计数正确
func TestManagerInsert(t *testing.T) {
	m := NewManager(Option{MaxSegmentSize: 2})
	d := doc.NewDoc("a")
	d.DocID = 1
	m.Insert(d)

	if m.DocCount() != 1 {
		t.Fatalf("expected 1, got %d", m.DocCount())
	}
}

// TestManagerRotate 验证 Segment Manager 超过容量自动轮转创建新 Segment
func TestManagerRotate(t *testing.T) {
	m := NewManager(Option{MaxSegmentSize: 2})
	for i := 0; i < 5; i++ {
		d := doc.NewDoc(string(rune('a' + i)))
		d.DocID = uint64(i)
		m.Insert(d)
	}
	if m.SegmentCount() != 3 {
		t.Fatalf("expected 3 segments, got %d", m.SegmentCount())
	}
}

// TestManagerDelete 验证 Segment Manager 删除文档后文档计数为0
func TestManagerDelete(t *testing.T) {
	m := NewManager(Option{MaxSegmentSize: 100})
	d := doc.NewDoc("doc1")
	d.DocID = 1
	m.Insert(d)

	if !m.Delete("doc1") {
		t.Fatal("expected delete to succeed")
	}
	if m.DocCount() != 0 {
		t.Fatalf("expected 0, got %d", m.DocCount())
	}
}

// TestManagerSaveLoad 验证 Segment Manager 序列化保存与反序列化加载
func TestManagerSaveLoad(t *testing.T) {
	m := NewManager(Option{MaxSegmentSize: 100})
	d1 := doc.NewDoc("doc1")
	d1.DocID = 1
	d1.SetStringField("name", "test")
	m.Insert(d1)

	dir := filepath.Join(os.TempDir(), "go-zvec-mgr-test")
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	m2, err := LoadManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m2.DocCount() != 1 {
		t.Fatalf("expected 1, got %d", m2.DocCount())
	}
}
