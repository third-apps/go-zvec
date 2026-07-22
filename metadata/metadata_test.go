package metadata

import (
	"testing"
)

// TestMetadataIndexString 验证元数据索引字符串类型匹配
func TestMetadataIndexString(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddString("tenant", 1, "acme")
	idx.AddString("tenant", 2, "acme")
	idx.AddString("tenant", 3, "globex")

	result := idx.MatchString("tenant", "acme")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}

	result = idx.MatchString("tenant", "globex")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}

	result = idx.MatchString("tenant", "nonexistent")
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

// TestMetadataIndexInt64 验证元数据索引 Int64 类型匹配
func TestMetadataIndexInt64(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddInt64("year", 1, 2024)
	idx.AddInt64("year", 2, 2024)
	idx.AddInt64("year", 3, 2025)

	result := idx.MatchInt64("year", 2024)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

// TestMetadataIndexBool 验证元数据索引布尔类型匹配
func TestMetadataIndexBool(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddBool("active", 1, true)
	idx.AddBool("active", 2, false)

	result := idx.MatchBool("active", true)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
}

// TestMetadataIndexDeleteDoc 验证元数据索引按文档 ID 删除后匹配数减少
func TestMetadataIndexDeleteDoc(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddString("tenant", 1, "acme")
	idx.AddString("tenant", 2, "acme")
	idx.DeleteDoc(1)

	result := idx.MatchString("tenant", "acme")
	if len(result) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(result))
	}
}

// TestMetadataIndexMatchStrings 验证元数据索引多值字符串匹配
func TestMetadataIndexMatchStrings(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddString("category", 1, "go")
	idx.AddString("category", 2, "rust")
	idx.AddString("category", 3, "python")

	result := idx.MatchStrings("category", []string{"go", "rust"})
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

// TestMetadataIndexFieldValues 验证元数据索引获取字段所有值
func TestMetadataIndexFieldValues(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddString("lang", 1, "go")
	idx.AddString("lang", 2, "rust")

	vals := idx.FieldValues("lang")
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}
}

// TestMetadataIndexDocCount 验证元数据索引文档计数统计
func TestMetadataIndexDocCount(t *testing.T) {
	idx := NewMetadataIndex()
	idx.AddString("tenant", 1, "acme")
	idx.AddString("tenant", 2, "acme")
	idx.AddString("tenant", 3, "globex")

	if idx.DocCount("tenant", "acme") != 2 {
		t.Fatalf("expected 2, got %d", idx.DocCount("tenant", "acme"))
	}
}

// TestMetadataIndexNonExistentField 验证元数据索引查询不存在的字段返回 nil
func TestMetadataIndexNonExistentField(t *testing.T) {
	idx := NewMetadataIndex()
	result := idx.MatchString("nonexistent", "value")
	if result != nil {
		t.Fatalf("expected nil for nonexistent field")
	}
}
