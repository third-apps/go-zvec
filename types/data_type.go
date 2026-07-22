package types

type DataType uint32

const (
	DataTypeUndefined DataType = 0

	// Scalar types
	DataTypeBinary DataType = 1
	DataTypeString DataType = 2
	DataTypeBool   DataType = 3
	DataTypeInt32  DataType = 4
	DataTypeInt64  DataType = 5
	DataTypeUint32 DataType = 6
	DataTypeUint64 DataType = 7
	DataTypeFloat  DataType = 8
	DataTypeDouble DataType = 9

	// Dense vector types
	DataTypeVectorBinary32 DataType = 20
	DataTypeVectorBinary64 DataType = 21
	DataTypeVectorFP16     DataType = 22
	DataTypeVectorFP32     DataType = 23
	DataTypeVectorFP64     DataType = 24
	DataTypeVectorInt4     DataType = 25
	DataTypeVectorInt8     DataType = 26
	DataTypeVectorInt16    DataType = 27

	// Sparse vector types
	DataTypeSparseVectorFP16 DataType = 30
	DataTypeSparseVectorFP32 DataType = 31

	// Array types
	DataTypeArrayBinary DataType = 40
	DataTypeArrayString DataType = 41
	DataTypeArrayBool   DataType = 42
	DataTypeArrayInt32  DataType = 43
	DataTypeArrayInt64  DataType = 44
	DataTypeArrayUint32 DataType = 45
	DataTypeArrayUint64 DataType = 46
	DataTypeArrayFloat  DataType = 47
	DataTypeArrayDouble DataType = 48
)

func (t DataType) IsVector() bool {
	return t.IsDenseVector() || t.IsSparseVector()
}

func (t DataType) IsDenseVector() bool {
	return t >= DataTypeVectorBinary32 && t <= DataTypeVectorInt16
}

func (t DataType) IsSparseVector() bool {
	return t == DataTypeSparseVectorFP16 || t == DataTypeSparseVectorFP32
}

func (t DataType) IsArray() bool {
	return t >= DataTypeArrayBinary && t <= DataTypeArrayDouble
}

func (t DataType) IsScalar() bool {
	return t >= DataTypeBinary && t <= DataTypeDouble
}

func (t DataType) String() string {
	switch t {
	case DataTypeUndefined:
		return "undefined"
	case DataTypeBinary:
		return "binary"
	case DataTypeString:
		return "string"
	case DataTypeBool:
		return "bool"
	case DataTypeInt32:
		return "int32"
	case DataTypeInt64:
		return "int64"
	case DataTypeUint32:
		return "uint32"
	case DataTypeUint64:
		return "uint64"
	case DataTypeFloat:
		return "float"
	case DataTypeDouble:
		return "double"
	case DataTypeVectorBinary32:
		return "vector_binary32"
	case DataTypeVectorBinary64:
		return "vector_binary64"
	case DataTypeVectorFP16:
		return "vector_fp16"
	case DataTypeVectorFP32:
		return "vector_fp32"
	case DataTypeVectorFP64:
		return "vector_fp64"
	case DataTypeVectorInt4:
		return "vector_int4"
	case DataTypeVectorInt8:
		return "vector_int8"
	case DataTypeVectorInt16:
		return "vector_int16"
	case DataTypeSparseVectorFP16:
		return "sparse_vector_fp16"
	case DataTypeSparseVectorFP32:
		return "sparse_vector_fp32"
	case DataTypeArrayBinary:
		return "array_binary"
	case DataTypeArrayString:
		return "array_string"
	case DataTypeArrayBool:
		return "array_bool"
	case DataTypeArrayInt32:
		return "array_int32"
	case DataTypeArrayInt64:
		return "array_int64"
	case DataTypeArrayUint32:
		return "array_uint32"
	case DataTypeArrayUint64:
		return "array_uint64"
	case DataTypeArrayFloat:
		return "array_float"
	case DataTypeArrayDouble:
		return "array_double"
	default:
		return "unknown"
	}
}
