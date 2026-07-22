package query

import "testing"

// TestNewMetadataFilter 验证创建空的元数据过滤器
func TestNewMetadataFilter(t *testing.T) {
	f := NewMetadataFilter()
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(f.Conditions) != 0 {
		t.Fatal("expected empty conditions")
	}
}

// TestMetadataFilterWhereEq 验证元数据过滤条件：字符串等于
func TestMetadataFilterWhereEq(t *testing.T) {
	f := NewMetadataFilter().WhereEq("tenant", "acme")
	if len(f.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(f.Conditions))
	}
	c := f.Conditions[0]
	if c.FieldName != "tenant" || c.Op != MetadataOpEq || c.StringVal != "acme" {
		t.Fatalf("unexpected condition: %+v", c)
	}
}

// TestMetadataFilterWhereIn 验证元数据过滤条件：字符串 IN 集合
func TestMetadataFilterWhereIn(t *testing.T) {
	f := NewMetadataFilter().WhereIn("status", []string{"active", "pending"})
	if len(f.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(f.Conditions))
	}
	c := f.Conditions[0]
	if c.Op != MetadataOpIn || len(c.StringVals) != 2 {
		t.Fatalf("unexpected condition: %+v", c)
	}
}

// TestMetadataFilterWhereIntEq 验证元数据过滤条件：整数等于
func TestMetadataFilterWhereIntEq(t *testing.T) {
	f := NewMetadataFilter().WhereIntEq("year", 2026)
	if len(f.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(f.Conditions))
	}
	c := f.Conditions[0]
	if c.Int64Val != 2026 {
		t.Fatalf("expected 2026, got %d", c.Int64Val)
	}
}

// TestMetadataFilterWhereBoolEq 验证元数据过滤条件：布尔等于
func TestMetadataFilterWhereBoolEq(t *testing.T) {
	f := NewMetadataFilter().WhereBoolEq("active", true)
	if len(f.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(f.Conditions))
	}
	c := f.Conditions[0]
	if !c.BoolVal {
		t.Fatal("expected true")
	}
}

// TestMetadataFilterWhereIntGtLt 验证元数据过滤条件：整数大于和小于
func TestMetadataFilterWhereIntGtLt(t *testing.T) {
	f := NewMetadataFilter().WhereIntGt("age", 18).WhereIntLt("age", 65)
	if len(f.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(f.Conditions))
	}
	if f.Conditions[0].Op != MetadataOpGt {
		t.Fatal("expected Gt")
	}
	if f.Conditions[1].Op != MetadataOpLt {
		t.Fatal("expected Lt")
	}
}

// TestMetadataFilterChaining 验证元数据过滤条件链式调用
func TestMetadataFilterChaining(t *testing.T) {
	f := NewMetadataFilter().
		WhereEq("tenant", "acme").
		WhereIntGt("score", 90).
		WhereBoolEq("verified", true)
	if len(f.Conditions) != 3 {
		t.Fatalf("expected 3 conditions, got %d", len(f.Conditions))
	}
}
