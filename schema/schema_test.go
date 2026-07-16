package schema

import (
	"testing"

	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/types"
)

func TestFieldSchemaValidate_Valid(t *testing.T) {
	f := NewFieldSchema("name", types.DataTypeString, false, 0)
	if st := f.Validate(); !st.OK() {
		t.Fatalf("expected OK, got: %v", st.Message())
	}
}

func TestFieldSchemaValidate_EmptyName(t *testing.T) {
	f := NewFieldSchema("", types.DataTypeString, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for empty name")
	}
}

func TestFieldSchemaValidate_UndefinedType(t *testing.T) {
	f := NewFieldSchema("f", types.DataTypeUndefined, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for undefined type")
	}
}

func TestFieldSchemaValidate_VectorZeroDimension(t *testing.T) {
	f := NewFieldSchema("vec", types.DataTypeVectorFP32, false, 0)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for zero dimension vector")
	}
}

func TestFieldSchemaValidate_VectorNegativeDimension(t *testing.T) {
	f := NewFieldSchema("vec", types.DataTypeVectorFP32, false, -1)
	if st := f.Validate(); st.OK() {
		t.Fatal("expected error for negative dimension vector")
	}
}

func TestFieldSchemaValidate_ScalarZeroDimension(t *testing.T) {
	f := NewFieldSchema("age", types.DataTypeInt32, false, 0)
	if st := f.Validate(); !st.OK() {
		t.Fatalf("expected OK for scalar with zero dim, got: %v", st.Message())
	}
}

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

func TestCollectionSchemaAddField_Nil(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AddField(nil); st.OK() {
		t.Fatal("expected error for nil field")
	}
}

func TestCollectionSchemaAddField_Duplicate(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("name", types.DataTypeString, false, 0))
	if st := s.AddField(NewFieldSchema("name", types.DataTypeInt32, false, 0)); st.OK() {
		t.Fatal("expected error for duplicate field")
	}
}

func TestCollectionSchemaAddField_Invalid(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AddField(NewFieldSchema("", types.DataTypeString, false, 0)); st.OK() {
		t.Fatal("expected error for invalid field")
	}
}

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

func TestCollectionSchemaAlterField_NotFound(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.AlterField("nonexist", "new", nil); st.OK() {
		t.Fatal("expected error for non-existent field")
	}
}

func TestCollectionSchemaAlterField_InvalidNewField(t *testing.T) {
	s := NewCollectionSchema("test")
	s.AddField(NewFieldSchema("old", types.DataTypeString, false, 0))
	if st := s.AlterField("old", "new", NewFieldSchema("new", types.DataTypeUndefined, false, 0)); st.OK() {
		t.Fatal("expected error for invalid new field")
	}
}

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

func TestCollectionSchemaDropField_NotFound(t *testing.T) {
	s := NewCollectionSchema("test")
	if st := s.DropField("nonexist"); st.OK() {
		t.Fatal("expected error for non-existent field")
	}
}

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
