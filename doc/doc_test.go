package doc

import (
	"testing"

	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

func TestDocValidate_ValidDoc(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("id", types.DataTypeString, false, 0))
	s.AddField(schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 4))

	d := NewDoc("doc1")
	d.SetStringField("id", "doc1")
	d.SetVectorFP32Field("vec", []float32{0.1, 0.2, 0.3, 0.4})

	if err := d.Validate(s); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDocValidate_EmptyID(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("id", types.DataTypeString, false, 0))

	d := NewDoc("")
	if err := d.Validate(s); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestDocValidate_MissingField(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("name", types.DataTypeString, false, 0))

	d := NewDoc("doc1")
	if err := d.Validate(s); err == nil {
		t.Fatal("expected error for missing field")
	}
}

func TestDocValidate_NullableField(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("name", types.DataTypeString, true, 0))

	d := NewDoc("doc1")
	if err := d.Validate(s); err != nil {
		t.Fatalf("expected no error for nullable field, got: %v", err)
	}
}

func TestDocValidate_MissingVector(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 4))

	d := NewDoc("doc1")
	if err := d.Validate(s); err == nil {
		t.Fatal("expected error for missing vector field")
	}
}

func TestDocValidate_NullableVector(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("vec", types.DataTypeVectorFP32, true, 4))

	d := NewDoc("doc1")
	if err := d.Validate(s); err != nil {
		t.Fatalf("expected no error for nullable vector, got: %v", err)
	}
}

func TestDocValidate_VectorDimensionMismatch(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 4))

	d := NewDoc("doc1")
	d.SetVectorFP32Field("vec", []float32{0.1, 0.2, 0.3})

	if err := d.Validate(s); err == nil {
		t.Fatal("expected error for dimension mismatch")
	}
}

func TestDocValidate_MultipleFields(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	s.AddField(schema.NewFieldSchema("id", types.DataTypeString, false, 0))
	s.AddField(schema.NewFieldSchema("age", types.DataTypeInt32, false, 0))
	s.AddField(schema.NewFieldSchema("vec", types.DataTypeVectorFP32, false, 2))

	d := NewDoc("doc1")
	d.SetStringField("id", "doc1")
	d.SetInt32Field("age", 30)
	d.SetVectorFP32Field("vec", []float32{0.5, 0.6})

	if err := d.Validate(s); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDocValidate_UnknownField(t *testing.T) {
	s := schema.NewCollectionSchema("test")
	d := NewDoc("doc1")
	if err := d.Validate(s); err != nil {
		t.Fatalf("expected no error for empty schema, got: %v", err)
	}
}

func TestDocGetters(t *testing.T) {
	d := NewDoc("doc1")
	d.SetStringField("name", "alice")
	d.SetVectorFP32Field("vec", []float32{1, 2, 3})
	d.SetSparseVector("sparse", SparseVectorValue{Indices: []uint32{0, 5}, Values: []float32{0.1, 0.9}})

	if !d.HasField("name") {
		t.Fatal("expected HasField true")
	}
	if d.HasField("nonexist") {
		t.Fatal("expected HasField false")
	}

	if !d.HasVector("vec") {
		t.Fatal("expected HasVector true")
	}
	if d.HasVector("nonexist") {
		t.Fatal("expected HasVector false")
	}

	if !d.HasSparseVector("sparse") {
		t.Fatal("expected HasSparseVector true")
	}

	v, ok := d.Field("name")
	if !ok || v.StringVal != "alice" {
		t.Fatal("Field getter failed")
	}

	vv, ok := d.Vector("vec")
	if !ok || len(vv.Float32s) != 3 {
		t.Fatal("Vector getter failed")
	}

	sv, ok := d.SparseVector("sparse")
	if !ok || len(sv.Indices) != 2 {
		t.Fatal("SparseVector getter failed")
	}

	names := d.FieldNames()
	if len(names) != 1 || names[0] != "name" {
		t.Fatal("FieldNames failed")
	}

	vnames := d.VectorNames()
	if len(vnames) != 1 || vnames[0] != "vec" {
		t.Fatal("VectorNames failed")
	}

	f32, ok := d.Vector("vec")
	if f32s, ok2 := f32.Float32(); !ok2 || len(f32s) != 3 {
		t.Fatal("expected Float32 success")
	}
	_ = ok
	_, ok2 := f32.Float64()
	if ok2 {
		t.Fatal("expected Float64 false for Float32 vector")
	}
}
