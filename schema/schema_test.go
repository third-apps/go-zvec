package schema

import (
	"testing"

	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/types"
)

// TestFieldSchemaValidate_Valid 验证合法字段 Schema 校验通过
func TestFieldSchemaValidate_Valid(t *testing.T) {
	f := NewFieldSchema("name", types.DataTypeString, false, 0)
	if st := f.Validate(); !st.OK() {
		t.Fatalf("expected OK, got: %v", st.Message())
	}
}

// TestFieldSchemaValidate_EmptyName 验证空名称字段校验失败
func TestFieldSchemaValidate_EmptyName(t *testing.T) {
	f := NewFieldSchema("", types.DataTypeString, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for empty name")
	}
}

// TestFieldSchemaValidate_UndefinedType 验证未定义类型字段校验失败
func TestFieldSchemaValidate_UndefinedType(t *testing.T) {
	f := NewFieldSchema("f", types.DataTypeUndefined, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for undefined type")
	}
}

// TestFieldSchemaValidate_VectorZeroDimension 验证零维度向量字段校验失败
func TestFieldSchemaValidate_VectorZeroDimension(t *testing.T) {
	f := NewFieldSchema("vec", types.DataTypeVectorFP32, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for zero dimension vector")
	}
}

// TestFieldSchemaValidate_VectorNegativeDimension 验证负维度向量字段校验失败
func TestFieldSchemaValidate_VectorNegativeDimension(t *testing.T) {
	f := NewFieldSchema("vec", types.DataTypeVectorFP32, false, -1)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for negative dimension vector")
	}
}

// TestFieldSchemaValidate_ScalarZeroDimension 验证标量字段零维度校验通过
func TestFieldSchemaValidate_ScalarZeroDimension(t *testing.T) {
	f := NewFieldSchema("age", types.DataTypeInt32, false, 0)
	if st := f.Validate(); !st.OK() {
		t.Fatalf("expected OK for scalar with zero dim, got: %v", st.Message())
	}
}

// TestCollectionSchemaAddField_Valid 验证集合 Schema 添加合法字段
func TestCollectionSchemaAddField_Valid(t *testing.T) {
	s := NewCollectionSchema("test")
	f := NewFieldSchema("name", types.DataTypeString, false, 0)
	if st := s.AddField(f); !st.OK() {
		t.Fatalf("expected OK, got: %v", st.Message())
	}
	if !s.HasField("name") {
		t.Fatal("expected HasField true")
	}
	if s.GetField("name") == nil {
		t.Fatal("expected GetField non-nil")
	}
}

// TestCollectionSchemaAddField_Nil 验证集合 Schema 添加 nil 字段校验失败
func TestCollectionSchemaAddField_Nil(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AddField(nil); st.OK() {
		t.Fatal("expected error for nil field")
	}
}

// TestCollectionSchemaAddField_Duplicate 验证集合 Schema 添加重复字段校验失败
func TestCollectionSchemaAddField_Duplicate(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("name", types.DataTypeString, false, 0))
	if st := s.AddField(NewFieldSchema("name", types.DataTypeInt32, false, 0)); st.OK() {
		t.Fatal("expected error for duplicate field")
	}
}

// TestCollectionSchemaAddField_Invalid 验证集合 Schema 添加无效字段校验失败
func TestCollectionSchemaAddField_Invalid(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AddField(NewFieldSchema("", types.DataTypeString, false, 0)); st.OK() {
		t.Fatal("expected error for invalid field")
	}
}

// TestCollectionSchemaAlterField_Valid 验证集合 Schema 修改字段名称和定义
func TestCollectionSchemaAlterField_Valid(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("old", types.DataTypeString, false, 0))
	newF := NewFieldSchema("new", types.DataTypeString, false, 0)
	if st := s.AlterField("old", "new", newF); !st.OK() {
		t.Fatalf("expected OK, got: %v", st.Message())
	}
	if s.HasField("old") {
		t.Fatal("expected old field removed")
	}
	if !s.HasField("new") {
		t.Fatal("expected new field added")
	}
}

// TestCollectionSchemaAlterField_NotFound 验证集合 Schema 修改不存在字段校验失败
func TestCollectionSchemaAlterField_NotFound(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AlterField("nonexist", "new", nil); st.OK() {
		t.Fatal("expected error for non-existent field")
	}
}

// TestCollectionSchemaAlterField_InvalidNewField 验证集合 Schema 修改为无效字段校验失败
func TestCollectionSchemaAlterField_InvalidNewField(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("old", types.DataTypeString, false, 0))
	if st := s.AlterField("old", "new", NewFieldSchema("new", types.DataTypeUndefined, false, 0)); st.OK() {
		t.Fatal("expected error for invalid new field")
	}
}

// TestCollectionSchemaAlterField_NilField 验证集合 Schema 修改字段时 nil 新字段仅重命名
func TestCollectionSchemaAlterField_NilField(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("old", types.DataTypeString, false, 0))
	if st := s.AlterField("old", "new", nil); !st.OK() {
		t.Fatalf("expected OK for nil new field, got: %v", st.Message())
	}
	if s.HasField("old") {
		t.Fatal("expected old field removed after rename")
	}
	if !s.HasField("new") {
		t.Fatal("expected new field present after rename")
	}
}

// TestCollectionSchemaDropField_Valid 验证集合 Schema 删除字段功能
func TestCollectionSchemaDropField_Valid(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("name", types.DataTypeString, false, 0))
	s.AddField(NewFieldSchema("age", types.DataTypeInt32, false, 0))
	if st := s.DropField("name"); !st.OK() {
		t.Fatalf("expected OK, got: %v", st.Message())
	}
	if s.HasField("name") {
		t.Fatal("expected field removed")
	}
	if len(s.Fields()) != 1 || s.Fields()[0].Name != "age" {
		t.Fatal("expected only age remaining in order")
	}
}

// TestCollectionSchemaDropField_NotFound 验证集合 Schema 删除不存在字段校验失败
func TestCollectionSchemaDropField_NotFound(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.DropField("nonexist"); st.OK() {
		t.Fatal("expected error for non-existent field")
	}
}

// TestCollectionSchemaFieldOrder 验证集合 Schema 字段添加顺序保持
func TestCollectionSchemaFieldOrder(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("a", types.DataTypeString, false, 0))
	s.AddField(NewFieldSchema("b", types.DataTypeInt32, false, 0))
	s.AddField(NewFieldSchema("c", types.DataTypeFloat, false, 0))
	fields := s.Fields()
	if len(fields) != 3 || fields[0].Name != "a" || fields[1].Name != "b" || fields[2].Name != "c" {
		t.Fatal("field order should be a, b, c")
	}
}

// TestCollectionSchemaFilterMethods 验证集合 Schema 向量字段、FTS 字段等过滤方法
func TestCollectionSchemaFilterMethods(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("name", types.DataTypeString, false, 0))
	s.AddField(NewFieldSchema("vec", types.DataTypeVectorFP32, false, 4))
	s.AddField(NewFieldSchema("sparse", types.DataTypeSparseVectorFP32, false, 4))

	vecF := NewFieldSchema("fts_f", types.DataTypeString, false, 0)
	vecF.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(vecF)

	if len(s.VectorFields()) != 2 {
		t.Fatal("expected 2 vector fields")
	}
	if len(s.FTSFields()) != 1 {
		t.Fatal("expected 1 FTS field")
	}
	if len(s.ForwardFields()) != 2 {
		t.Fatal("expected 2 forward fields (name + fts_f)")
	}
	if len(s.AllFieldNames()) != 4 {
		t.Fatal("expected 4 field names")
	}
}

// TestFieldSchemaElementDataType 验证数组类型的元素数据类型推导
func TestFieldSchemaElementDataType(t *testing.T) {
	tests := []struct {
		dt     types.DataType
		expect types.DataType
	}{
		{types.DataTypeArrayInt32, types.DataTypeInt32},
		{types.DataTypeArrayInt64, types.DataTypeInt64},
		{types.DataTypeArrayFloat, types.DataTypeFloat},
		{types.DataTypeArrayDouble, types.DataTypeDouble},
		{types.DataTypeArrayString, types.DataTypeString},
		{types.DataTypeArrayBool, types.DataTypeBool},
		{types.DataTypeArrayBinary, types.DataTypeBinary},
		{types.DataTypeArrayUint32, types.DataTypeUint32},
		{types.DataTypeArrayUint64, types.DataTypeUint64},
		{types.DataTypeString, types.DataTypeString},
	}

	for _, tc := range tests {
		f := NewFieldSchema("f", tc.dt, false, 0)
		if got := f.ElementDataType(); got != tc.expect {
			t.Errorf("ElementDataType(%v) = %v, want %v", tc.dt, got, tc.expect)
		}
	}
}

// TestFieldSchemaElementDataSize 验证数组类型的元素字节大小
func TestFieldSchemaElementDataSize(t *testing.T) {
	tests := []struct {
		dt     types.DataType
		expect int
	}{
		{types.DataTypeArrayInt32, 4},
		{types.DataTypeArrayUint32, 4},
		{types.DataTypeArrayInt64, 8},
		{types.DataTypeArrayUint64, 8},
		{types.DataTypeArrayFloat, 4},
		{types.DataTypeArrayDouble, 8},
		{types.DataTypeArrayBool, 1},
		{types.DataTypeArrayBinary, 1},
		{types.DataTypeString, 0},
	}

	for _, tc := range tests {
		f := NewFieldSchema("f", tc.dt, false, 0)
		if got := f.ElementDataSize(); got != tc.expect {
			t.Errorf("ElementDataSize(%v) = %d, want %d", tc.dt, got, tc.expect)
		}
	}
}

// TestHasIndex 验证集合 Schema 判断字段是否已建索引
func TestHasIndex(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("name", types.DataTypeString, false, 0))
	if s.HasIndex("name") {
		t.Fatal("expected no index")
	}
	f := NewFieldSchema("fts_f", types.DataTypeString, false, 0)
	f.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
	s.AddField(f)
	if !s.HasIndex("fts_f") {
		t.Fatal("expected index present")
	}
}

// TestCollectionSchemaDefaults 验证新建集合 Schema 默认值
func TestCollectionSchemaDefaults(t *testing.T) {
	s := NewCollectionSchema("test")
	if s.Name != "test" {
		t.Fatal("name mismatch")
	}
	fields := s.Fields()
	if len(fields) != 0 {
		t.Fatal("expected empty fields")
	}
}
