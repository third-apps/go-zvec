package invert

import (
	"testing"
)

// TestInvertIndexAdd 验证倒排索引添加词条和搜索功能
func TestInvertIndexAdd(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	idx.Add(2, "golang")
	idx.Add(3, "rust")

	result := idx.Search("golang")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

// TestInvertIndexDelete 验证倒排索引删除单词条功能
func TestInvertIndexDelete(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	idx.Add(2, "golang")
	idx.Delete(1, "golang")

	result := idx.Search("golang")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
}

// TestInvertIndexDeleteDoc 验证倒排索引按文档 ID 删除全部词条
func TestInvertIndexDeleteDoc(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	idx.Add(1, "rust")
	idx.DeleteDoc(1)

	if idx.Search("golang") != nil || idx.Search("rust") != nil {
		t.Fatal("expected nil after DeleteDoc")
	}
}

// TestInvertIndexSearchWithFilter 验证倒排索引带过滤条件搜索功能
func TestInvertIndexSearchWithFilter(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	idx.Add(2, "golang")

	result := idx.SearchWithFilter("golang", func(docID uint64) bool {
		return docID == 1
	})
	if len(result) != 1 || result[0] != 1 {
		t.Fatalf("expected [1], got %v", result)
	}
}

// TestInvertIndexEmptyValue 验证倒排索引空值词条搜索返回 nil
func TestInvertIndexEmptyValue(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "")
	result := idx.Search("")
	if result != nil {
		t.Fatal("expected nil for empty value")
	}
}

// TestInvertIndexHasValue 验证倒排索引判断词条是否存在
func TestInvertIndexHasValue(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	if !idx.HasValue("golang") {
		t.Fatal("expected true")
	}
	if idx.HasValue("rust") {
		t.Fatal("expected false")
	}
}

// TestInvertIndexSize 验证倒排索引文档数量统计
func TestInvertIndexSize(t *testing.T) {
	idx := NewInvertIndex()
	idx.Add(1, "golang")
	idx.Add(2, "rust")
	if idx.Size() != 2 {
		t.Fatalf("expected 2, got %d", idx.Size())
	}
}
