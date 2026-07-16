package schema

import (
	"encoding/json"
	"fmt"

	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/status"
	"github.com/third-apps/go-zvec/types"
)

type FieldSchema struct {
	Name       string
	DataType   types.DataType
	Nullable   bool
	Dimension  int
	IndexParam *param.IndexParams
}

func NewFieldSchema(name string, dataType types.DataType, nullable bool, dimension int) *FieldSchema {
	return &FieldSchema{
		Name: name, DataType: dataType,
		Nullable: nullable, Dimension: dimension,
	}
}

func (f *FieldSchema) SetIndexParams(p *param.IndexParams) {
	f.IndexParam = p
}

func (f *FieldSchema) IsVectorField() bool {
	return f.DataType.IsDenseVector() || f.DataType.IsSparseVector()
}

func (f *FieldSchema) IsDenseVector() bool {
	return f.DataType.IsDenseVector()
}

func (f *FieldSchema) IsSparseVector() bool {
	return f.DataType.IsSparseVector()
}

func (f *FieldSchema) HasInvertIndex() bool {
	return f.IndexParam != nil && f.IndexParam.Type == types.IndexTypeInvert
}

func (f *FieldSchema) IsArrayType() bool {
	return f.DataType.IsArray()
}

func (f *FieldSchema) IndexType() types.IndexType {
	if f.IndexParam != nil {
		return f.IndexParam.Type
	}
	return types.IndexTypeUndefined
}

func (f *FieldSchema) ElementDataType() types.DataType {
	switch f.DataType {
	case types.DataTypeArrayBinary:
		return types.DataTypeBinary
	case types.DataTypeArrayString:
		return types.DataTypeString
	case types.DataTypeArrayBool:
		return types.DataTypeBool
	case types.DataTypeArrayInt32:
		return types.DataTypeInt32
	case types.DataTypeArrayInt64:
		return types.DataTypeInt64
	case types.DataTypeArrayUint32:
		return types.DataTypeUint32
	case types.DataTypeArrayUint64:
		return types.DataTypeUint64
	case types.DataTypeArrayFloat:
		return types.DataTypeFloat
	case types.DataTypeArrayDouble:
		return types.DataTypeDouble
	default:
		return f.DataType
	}
}

func (f *FieldSchema) ElementDataSize() int {
	switch f.DataType {
	case types.DataTypeArrayBinary:
		return 1
	case types.DataTypeArrayBool:
		return 1
	case types.DataTypeArrayInt32, types.DataTypeArrayUint32:
		return 4
	case types.DataTypeArrayInt64, types.DataTypeArrayUint64:
		return 8
	case types.DataTypeArrayFloat:
		return 4
	case types.DataTypeArrayDouble:
		return 8
	default:
		return 0
	}
}

func (f *FieldSchema) Validate() status.Status {
	if f.Name == "" {
		return status.NewInvalidArgument("field name cannot be empty")
	}
	if f.DataType == types.DataTypeUndefined {
		return status.NewInvalidArgument("field data type cannot be undefined")
	}
	if f.IsVectorField() {
		if f.Dimension <= 0 {
			return status.NewInvalidArgument(fmt.Sprintf(
				"vector field '%s' must have positive dimension", f.Name))
		}
	}
	return status.OKStatus()
}

type CollectionSchema struct {
	Name                  string
	fields                map[string]*FieldSchema
	fieldOrder            []string
	maxDocCountPerSegment int
}

const (
	MaxDocCountPerSegment             = 10000000
	MaxDocCountPerSegmentMinThreshold = 1000
)

func NewCollectionSchema(name string) *CollectionSchema {
	return &CollectionSchema{
		Name:                  name,
		fields:                make(map[string]*FieldSchema),
		maxDocCountPerSegment: MaxDocCountPerSegment,
	}
}

func (s *CollectionSchema) AddField(field *FieldSchema) status.Status {
	if field == nil {
		return status.NewInvalidArgument("field cannot be nil")
	}
	if st := field.Validate(); !st.OK() {
		return st
	}
	if _, exists := s.fields[field.Name]; exists {
		return status.NewAlreadyExists(fmt.Sprintf("field '%s' already exists", field.Name))
	}
	s.fields[field.Name] = field
	s.fieldOrder = append(s.fieldOrder, field.Name)
	return status.OKStatus()
}

func (s *CollectionSchema) AlterField(oldName string, newName string, newField *FieldSchema) status.Status {
	if _, exists := s.fields[oldName]; !exists {
		return status.NewNotFound(fmt.Sprintf("field '%s' not found", oldName))
	}
	if newField != nil {
		if st := newField.Validate(); !st.OK() {
			return st
		}
		delete(s.fields, oldName)
		s.fields[newName] = newField
		for i, n := range s.fieldOrder {
			if n == oldName {
				s.fieldOrder[i] = newName
				break
			}
		}
	} else {
		if oldName == newName {
			return status.OKStatus()
		}
		s.fields[newName] = s.fields[oldName]
		delete(s.fields, oldName)
		for i, n := range s.fieldOrder {
			if n == oldName {
				s.fieldOrder[i] = newName
				break
			}
		}
	}
	return status.OKStatus()
}

func (s *CollectionSchema) DropField(name string) status.Status {
	if _, exists := s.fields[name]; !exists {
		return status.NewNotFound(fmt.Sprintf("field '%s' not found", name))
	}
	delete(s.fields, name)
	for i, n := range s.fieldOrder {
		if n == name {
			s.fieldOrder = append(s.fieldOrder[:i], s.fieldOrder[i+1:]...)
			break
		}
	}
	return status.OKStatus()
}

func (s *CollectionSchema) HasField(name string) bool {
	_, exists := s.fields[name]
	return exists
}

func (s *CollectionSchema) GetField(name string) *FieldSchema {
	return s.fields[name]
}

func (s *CollectionSchema) HasIndex(name string) bool {
	f, exists := s.fields[name]
	return exists && f.IndexParam != nil
}

func (s *CollectionSchema) Fields() []*FieldSchema {
	result := make([]*FieldSchema, 0, len(s.fieldOrder))
	for _, name := range s.fieldOrder {
		result = append(result, s.fields[name])
	}
	return result
}

func (s *CollectionSchema) ForwardFields() []*FieldSchema {
	var result []*FieldSchema
	for _, name := range s.fieldOrder {
		f := s.fields[name]
		if !f.IsVectorField() {
			result = append(result, f)
		}
	}
	return result
}

func (s *CollectionSchema) ForwardFieldsWithIndex() []*FieldSchema {
	var result []*FieldSchema
	for _, name := range s.fieldOrder {
		f := s.fields[name]
		if !f.IsVectorField() && f.HasInvertIndex() {
			result = append(result, f)
		}
	}
	return result
}

func (s *CollectionSchema) InvertFields() []*FieldSchema {
	var result []*FieldSchema
	for _, name := range s.fieldOrder {
		f := s.fields[name]
		if f.HasInvertIndex() {
			result = append(result, f)
		}
	}
	return result
}

func (s *CollectionSchema) FTSFields() []*FieldSchema {
	var result []*FieldSchema
	for _, name := range s.fieldOrder {
		f := s.fields[name]
		if f.IndexType() == types.IndexTypeFTS {
			result = append(result, f)
		}
	}
	return result
}

func (s *CollectionSchema) VectorFields() []*FieldSchema {
	var result []*FieldSchema
	for _, name := range s.fieldOrder {
		f := s.fields[name]
		if f.IsVectorField() {
			result = append(result, f)
		}
	}
	return result
}

func (s *CollectionSchema) AllFieldNames() []string {
	result := make([]string, len(s.fieldOrder))
	copy(result, s.fieldOrder)
	return result
}

func (s *CollectionSchema) MarshalJSON() ([]byte, error) {
	type alias struct {
		Name   string         `json:"name"`
		Fields []*FieldSchema `json:"fields"`
	}
	return json.Marshal(alias{Name: s.Name, Fields: s.Fields()})
}

func (s *CollectionSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		Name   string         `json:"name"`
		Fields []*FieldSchema `json:"fields"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	s.Name = a.Name
	s.fields = make(map[string]*FieldSchema)
	s.fieldOrder = make([]string, 0, len(a.Fields))
	for _, f := range a.Fields {
		s.fields[f.Name] = f
		s.fieldOrder = append(s.fieldOrder, f.Name)
	}
	return nil
}

type CollectionStats struct {
	DocCount          uint64
	IndexCompleteness map[string]float32
}
