package doc

import (
	"errors"
	"fmt"
	"sort"

	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

type Value struct {
	Null      bool
	Type      types.DataType
	BoolVal   bool
	Int32Val  int32
	Uint32Val uint32
	Int64Val  int64
	Uint64Val uint64
	FloatVal  float32
	DoubleVal float64
	StringVal string
	BinaryVal []byte
}

type VectorValue struct {
	Float32s []float32
	Float64s []float64
	Int8s    []int8
	Int16s   []int16
	Int32s   []int32
}

type SparseVectorValue struct {
	Indices []uint32
	Values  []float32
}

type Doc struct {
	ID            string
	Score         float32
	DocID         uint64
	fields        map[string]Value
	vectors       map[string]VectorValue
	sparseVectors map[string]SparseVectorValue
}

func NewDoc(id string) *Doc {
	return &Doc{
		ID:            id,
		fields:        make(map[string]Value),
		vectors:       make(map[string]VectorValue),
		sparseVectors: make(map[string]SparseVectorValue),
	}
}

func (d *Doc) HasField(name string) bool {
	_, ok := d.fields[name]
	return ok
}

func (d *Doc) HasVector(name string) bool {
	_, ok := d.vectors[name]
	return ok
}

func (d *Doc) HasSparseVector(name string) bool {
	_, ok := d.sparseVectors[name]
	return ok
}

func (d *Doc) Field(name string) (Value, bool) {
	v, ok := d.fields[name]
	return v, ok
}

func (d *Doc) Vector(name string) (VectorValue, bool) {
	v, ok := d.vectors[name]
	return v, ok
}

func (d *Doc) SparseVector(name string) (SparseVectorValue, bool) {
	v, ok := d.sparseVectors[name]
	return v, ok
}

func (d *Doc) FieldNames() []string {
	names := make([]string, 0, len(d.fields))
	for n := range d.fields {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (d *Doc) VectorNames() []string {
	names := make([]string, 0, len(d.vectors))
	for n := range d.vectors {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (d *Doc) SetField(name string, val Value) {
	d.fields[name] = val
}

func (d *Doc) SetVector(name string, val VectorValue) {
	d.vectors[name] = val
}

func (d *Doc) SetSparseVector(name string, val SparseVectorValue) {
	d.sparseVectors[name] = val
}

func (d *Doc) SetFieldNull(name string) {
	d.fields[name] = Value{Null: true, Type: types.DataTypeUndefined}
}

func (d *Doc) SetStringField(name, val string) {
	d.fields[name] = Value{Type: types.DataTypeString, StringVal: val}
}

func (d *Doc) SetInt32Field(name string, val int32) {
	d.fields[name] = Value{Type: types.DataTypeInt32, Int32Val: val}
}

func (d *Doc) SetInt64Field(name string, val int64) {
	d.fields[name] = Value{Type: types.DataTypeInt64, Int64Val: val}
}

func (d *Doc) SetUint32Field(name string, val uint32) {
	d.fields[name] = Value{Type: types.DataTypeUint32, Uint32Val: val}
}

func (d *Doc) SetUint64Field(name string, val uint64) {
	d.fields[name] = Value{Type: types.DataTypeUint64, Uint64Val: val}
}

func (d *Doc) SetFloatField(name string, val float32) {
	d.fields[name] = Value{Type: types.DataTypeFloat, FloatVal: val}
}

func (d *Doc) SetDoubleField(name string, val float64) {
	d.fields[name] = Value{Type: types.DataTypeDouble, DoubleVal: val}
}

func (d *Doc) SetBoolField(name string, val bool) {
	d.fields[name] = Value{Type: types.DataTypeBool, BoolVal: val}
}

func (d *Doc) SetVectorFP32Field(name string, val []float32) {
	d.vectors[name] = VectorValue{Float32s: val}
}

func (d *Doc) Validate(s *schema.CollectionSchema) error {
	if d.ID == "" {
		return errors.New("doc ID cannot be empty")
	}

	for _, field := range s.Fields() {
		if field.IsVectorField() {
			if field.IsDenseVector() {
				v, ok := d.vectors[field.Name]
				if !ok && field.Nullable {
					continue
				}
				if !ok {
					return fmt.Errorf("doc missing vector field '%s'", field.Name)
				}
				if len(v.Float32s) != field.Dimension {
					return fmt.Errorf("vector field '%s' dimension mismatch: expected %d, got %d",
						field.Name, field.Dimension, len(v.Float32s))
				}
			}
		} else {
			_, ok := d.fields[field.Name]
			if !ok && field.Nullable {
				continue
			}
			if !ok {
				return fmt.Errorf("doc missing field '%s'", field.Name)
			}
		}
	}
	return nil
}

func (v VectorValue) Float32() ([]float32, bool) {
	return v.Float32s, v.Float32s != nil
}

func (v VectorValue) Float64() ([]float64, bool) {
	return v.Float64s, v.Float64s != nil
}

func (v VectorValue) Int8() ([]int8, bool) {
	return v.Int8s, v.Int8s != nil
}

func (v SparseVectorValue) Empty() bool {
	return len(v.Indices) == 0
}
