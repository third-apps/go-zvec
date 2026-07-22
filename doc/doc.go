package doc

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/types"
)

// PackInt4s packs a slice of int8 values (each in range [-8, 7]) into
// a packed byte slice where each byte stores two int4 values.
// Returns the packed bytes and the original count.
func PackInt4s(values []int8) ([]byte, int) {
	n := len(values)
	if n == 0 {
		return nil, 0
	}
	packed := make([]byte, (n+1)/2)
	for i, v := range values {
		u := uint8(int8(v) & 0x0F)
		if i%2 == 0 {
			packed[i/2] = u
		} else {
			packed[i/2] |= u << 4
		}
	}
	return packed, n
}

// UnpackInt4s unpacks a packed byte slice into a slice of int8 values.
// count is the original number of int4 values (must be known from context).
func UnpackInt4s(data []byte, count int) []int8 {
	if count == 0 || len(data) == 0 {
		return nil
	}
	result := make([]int8, count)
	for i := 0; i < count; i++ {
		b := data[i/2]
		var u uint8
		if i%2 == 0 {
			u = b & 0x0F
		} else {
			u = b >> 4
		}
		result[i] = int8(u<<4) >> 4
	}
	return result
}

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
	Float32s  []float32
	Float64s  []float64
	Int8s     []int8
	Int16s    []int16
	Int32s    []int32
	Int4s     []byte // packed: two int4 values per byte
	Int4Count int    // original count of int4 values (needed for unpacking)
	Float16s  []uint16
}

// Int4sUnpacked returns the int4 values as a slice of int8 by unpacking.
func (v VectorValue) Int4sUnpacked() []int8 {
	return UnpackInt4s(v.Int4s, v.Int4Count)
}

// SetInt4s packs the given int8 values into the packed format.
func (v *VectorValue) SetInt4s(values []int8) {
	v.Int4s, v.Int4Count = PackInt4s(values)
}

// MarshalJSON implements json.Marshaler for backward-compatible JSON output.
func (v VectorValue) MarshalJSON() ([]byte, error) {
	type alias struct {
		Float32s []float32 `json:"float32s,omitempty"`
		Float64s []float64 `json:"float64s,omitempty"`
		Int8s    []int8    `json:"int8s,omitempty"`
		Int16s   []int16   `json:"int16s,omitempty"`
		Int32s   []int32   `json:"int32s,omitempty"`
		Int4s    []int8    `json:"int4s,omitempty"`
		Float16s []uint16  `json:"float16s,omitempty"`
	}
	return json.Marshal(alias{
		Float32s: v.Float32s,
		Float64s: v.Float64s,
		Int8s:    v.Int8s,
		Int16s:   v.Int16s,
		Int32s:   v.Int32s,
		Int4s:    v.Int4sUnpacked(),
		Float16s: v.Float16s,
	})
}

// UnmarshalJSON implements json.Unmarshaler for backward-compatible JSON input.
func (v *VectorValue) UnmarshalJSON(data []byte) error {
	type alias struct {
		Float32s []float32 `json:"float32s,omitempty"`
		Float64s []float64 `json:"float64s,omitempty"`
		Int8s    []int8    `json:"int8s,omitempty"`
		Int16s   []int16   `json:"int16s,omitempty"`
		Int32s   []int32   `json:"int32s,omitempty"`
		Int4s    []int8    `json:"int4s,omitempty"`
		Float16s []uint16  `json:"float16s,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	v.Float32s = a.Float32s
	v.Float64s = a.Float64s
	v.Int8s = a.Int8s
	v.Int16s = a.Int16s
	v.Int32s = a.Int32s
	v.Float16s = a.Float16s
	if a.Int4s != nil {
		v.SetInt4s(a.Int4s)
	}
	return nil
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

func (d *Doc) SparseVectorNames() []string {
	names := make([]string, 0, len(d.sparseVectors))
	for n := range d.sparseVectors {
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

// ForEachField calls fn for each field. Iteration order is non-deterministic.
func (d *Doc) ForEachField(fn func(name string, val Value)) {
	for n, v := range d.fields {
		fn(n, v)
	}
}

// ForEachVector calls fn for each vector. Iteration order is non-deterministic.
func (d *Doc) ForEachVector(fn func(name string, val VectorValue)) {
	for n, v := range d.vectors {
		fn(n, v)
	}
}

// ForEachSparseVector calls fn for each sparse vector. Iteration order is non-deterministic.
func (d *Doc) ForEachSparseVector(fn func(name string, val SparseVectorValue)) {
	for n, v := range d.sparseVectors {
		fn(n, v)
	}
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

func (d *Doc) SetBinaryField(name string, val []byte) {
	d.fields[name] = Value{Type: types.DataTypeBinary, BinaryVal: val}
}

func (d *Doc) SetVectorFP32Field(name string, val []float32) {
	d.vectors[name] = VectorValue{Float32s: val}
}

func (d *Doc) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID            string                       `json:"id"`
		Score         float32                      `json:"score"`
		DocID         uint64                       `json:"doc_id"`
		Fields        map[string]Value             `json:"fields,omitempty"`
		Vectors       map[string]VectorValue       `json:"vectors,omitempty"`
		SparseVectors map[string]SparseVectorValue `json:"sparse_vectors,omitempty"`
	}
	return json.Marshal(alias{
		ID:            d.ID,
		Score:         d.Score,
		DocID:         d.DocID,
		Fields:        d.fields,
		Vectors:       d.vectors,
		SparseVectors: d.sparseVectors,
	})
}

func (d *Doc) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID            string                       `json:"id"`
		Score         float32                      `json:"score"`
		DocID         uint64                       `json:"doc_id"`
		Fields        map[string]Value             `json:"fields,omitempty"`
		Vectors       map[string]VectorValue       `json:"vectors,omitempty"`
		SparseVectors map[string]SparseVectorValue `json:"sparse_vectors,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	d.ID = a.ID
	d.Score = a.Score
	d.DocID = a.DocID
	if a.Fields != nil {
		d.fields = a.Fields
	} else {
		d.fields = make(map[string]Value)
	}
	if a.Vectors != nil {
		d.vectors = a.Vectors
	} else {
		d.vectors = make(map[string]VectorValue)
	}
	if a.SparseVectors != nil {
		d.sparseVectors = a.SparseVectors
	} else {
		d.sparseVectors = make(map[string]SparseVectorValue)
	}
	return nil
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
				var dim int
				switch field.DataType {
				case types.DataTypeVectorFP32:
					dim = len(v.Float32s)
				case types.DataTypeVectorFP64:
					dim = len(v.Float64s)
				case types.DataTypeVectorInt8:
					dim = len(v.Int8s)
				case types.DataTypeVectorInt16:
					dim = len(v.Int16s)
				case types.DataTypeVectorInt4:
					dim = v.Int4Count
				case types.DataTypeVectorFP16:
					dim = len(v.Float16s)
				default:
					dim = len(v.Float32s)
				}
				if dim != field.Dimension {
					return fmt.Errorf("vector field '%s' dimension mismatch: expected %d, got %d",
						field.Name, field.Dimension, dim)
				}
			} else {
				sv, ok := d.sparseVectors[field.Name]
				if !ok && field.Nullable {
					continue
				}
				if !ok {
					return fmt.Errorf("doc missing sparse vector field '%s'", field.Name)
				}
				if len(sv.Indices) != len(sv.Values) {
					return fmt.Errorf("sparse vector field '%s' indices/values length mismatch: %d vs %d",
						field.Name, len(sv.Indices), len(sv.Values))
				}
			}
		} else {
			v, ok := d.fields[field.Name]
			if !ok && field.Nullable {
				continue
			}
			if !ok {
				return fmt.Errorf("doc missing field '%s'", field.Name)
			}
			if v.Null && !field.Nullable {
				return fmt.Errorf("non-nullable field '%s' cannot be null", field.Name)
			}
			if !v.Null && v.Type != field.DataType {
				return fmt.Errorf("field '%s' type mismatch: expected %v, got %v",
					field.Name, field.DataType, v.Type)
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
