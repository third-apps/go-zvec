package types

import "testing"

// TestDataTypeIsVector 验证 DataType 向量类型判断
func TestDataTypeIsVector(t *testing.T) {
	if DataTypeString.IsVector() {
		t.Fatal("string should not be vector")
	}
	if !DataTypeVectorFP32.IsVector() {
		t.Fatal("fp32 should be vector")
	}
	if !DataTypeVectorInt8.IsVector() {
		t.Fatal("int8 should be vector")
	}
}

// TestDataTypeIsDenseVector 验证 DataType 稠密向量类型判断
func TestDataTypeIsDenseVector(t *testing.T) {
	if DataTypeString.IsDenseVector() {
		t.Fatal("string should not be dense vector")
	}
	if !DataTypeVectorFP32.IsDenseVector() {
		t.Fatal("fp32 should be dense vector")
	}
	if DataTypeSparseVectorFP32.IsDenseVector() {
		t.Fatal("sparse should not be dense vector")
	}
}

// TestDataTypeIsSparseVector 验证 DataType 稀疏向量类型判断
func TestDataTypeIsSparseVector(t *testing.T) {
	if DataTypeVectorFP32.IsSparseVector() {
		t.Fatal("fp32 should not be sparse")
	}
	if !DataTypeSparseVectorFP32.IsSparseVector() {
		t.Fatal("sparse fp32 should be sparse")
	}
	if !DataTypeSparseVectorFP16.IsSparseVector() {
		t.Fatal("sparse fp16 should be sparse")
	}
}

// TestDataTypeIsArray 验证 DataType 数组类型判断
func TestDataTypeIsArray(t *testing.T) {
	if DataTypeString.IsArray() {
		t.Fatal("string should not be array")
	}
	if !DataTypeArrayString.IsArray() {
		t.Fatal("array_string should be array")
	}
	if !DataTypeArrayInt64.IsArray() {
		t.Fatal("array_int64 should be array")
	}
}

// TestDataTypeIsScalar 验证 DataType 标量类型判断
func TestDataTypeIsScalar(t *testing.T) {
	if DataTypeVectorFP32.IsScalar() {
		t.Fatal("vector should not be scalar")
	}
	if !DataTypeString.IsScalar() {
		t.Fatal("string should be scalar")
	}
	if !DataTypeInt64.IsScalar() {
		t.Fatal("int64 should be scalar")
	}
}

// TestDataTypeString 验证 DataType 字符串表示
func TestDataTypeString(t *testing.T) {
	if DataTypeVectorFP32.String() != "vector_fp32" {
		t.Fatalf("expected 'vector_fp32', got '%s'", DataTypeVectorFP32.String())
	}
	if DataTypeUndefined.String() != "undefined" {
		t.Fatalf("expected 'undefined', got '%s'", DataTypeUndefined.String())
	}
	if DataType(999).String() != "unknown" {
		t.Fatalf("expected 'unknown', got '%s'", DataType(999).String())
	}
}

// TestMetricTypeString 验证 MetricType 字符串表示
func TestMetricTypeString(t *testing.T) {
	if MetricTypeL2.String() != "l2" {
		t.Fatalf("expected 'l2', got '%s'", MetricTypeL2.String())
	}
	if MetricTypeCosine.String() != "cosine" {
		t.Fatalf("expected 'cosine', got '%s'", MetricTypeCosine.String())
	}
}

// TestIndexTypeString 验证 IndexType 字符串表示
func TestIndexTypeString(t *testing.T) {
	if IndexTypeHNSW.String() != "hnsw" {
		t.Fatalf("expected 'hnsw', got '%s'", IndexTypeHNSW.String())
	}
	if IndexTypeVamana.String() != "vamana" {
		t.Fatalf("expected 'vamana', got '%s'", IndexTypeVamana.String())
	}
}

// TestQuantizeTypeString 验证 QuantizeType 字符串表示
func TestQuantizeTypeString(t *testing.T) {
	if QuantizeTypeFP16.String() != "fp16" {
		t.Fatalf("expected 'fp16', got '%s'", QuantizeTypeFP16.String())
	}
	if QuantizeTypeRaBitQ.String() != "rabitq" {
		t.Fatalf("expected 'rabitq', got '%s'", QuantizeTypeRaBitQ.String())
	}
}
