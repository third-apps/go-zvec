package quantizer

import (
	"testing"
)

// TestFP16Quantizer 验证 FP16 量化器编解码精度和元数据
func TestFP16Quantizer(t *testing.T) {
	q := NewFP16Quantizer()
	vec := []float32{1.5, -2.0, 0.0, 3.14159}

	encoded := q.Encode(vec, nil)
	decoded := q.Decode(encoded, nil)

	if len(decoded) != len(vec) {
		t.Fatalf("expected %d values, got %d", len(vec), len(decoded))
	}

	for i := range vec {
		diff := decoded[i] - vec[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			t.Fatalf("value %d: expected ~%f, got %f (fp16 precision loss)", i, vec[i], decoded[i])
		}
	}

	if q.Type() != 1 {
		t.Fatalf("expected FP16 type (1), got %d", q.Type())
	}
	if q.CodeSize() != 2 {
		t.Fatalf("expected code size 2, got %d", q.CodeSize())
	}
}

// TestInt8Quantizer 验证 Int8 量化器训练和编解码精度
func TestInt8Quantizer(t *testing.T) {
	q := NewInt8Quantizer(3, false)

	vec := []float32{0.5, -0.3, 1.0}
	q.Train([][]float32{vec})

	encoded := q.Encode(vec, nil)
	decoded := q.Decode(encoded, nil)

	if len(decoded) != len(vec) {
		t.Fatalf("expected %d values, got %d", len(vec), len(decoded))
	}

	threshold := float32(0.1)
	for i := range vec {
		diff := decoded[i] - vec[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > threshold {
			t.Fatalf("value %d: expected ~%f, got %f (int8 error > %f)", i, vec[i], decoded[i], threshold)
		}
	}
}

// TestInt4Quantizer 验证 Int4 量化器训练和编解码精度
func TestInt4Quantizer(t *testing.T) {
	q := NewInt4Quantizer(4, false)

	vec := []float32{0.5, -0.5, 1.0, -1.0}
	q.Train([][]float32{vec})

	encoded := q.Encode(vec, nil)
	decoded := q.Decode(encoded, nil)

	if len(decoded) != len(vec) {
		t.Fatalf("expected %d values, got %d", len(vec), len(decoded))
	}

	for i := range vec {
		diff := decoded[i] - vec[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.2 {
			t.Fatalf("value %d: expected %f, got %f (int4 error > 0.2)", i, vec[i], decoded[i])
		}
	}
}

// TestInt8QuantizerWithRotation 验证 Int8 量化器带随机旋转的编解码
func TestInt8QuantizerWithRotation(t *testing.T) {
	q := NewInt8Quantizer(4, true)
	vec := []float32{0.5, -0.3, 0.8, -0.1}
	q.Train([][]float32{vec})

	encoded := q.Encode(vec, nil)
	decoded := q.Decode(encoded, nil)

	if len(decoded) != len(vec) {
		t.Fatalf("expected %d values, got %d", len(vec), len(decoded))
	}
}

// TestFloat16Conversions 验证 float32 与 float16 双向转换正确性
func TestFloat16Conversions(t *testing.T) {
	tests := []float32{0.0, 1.0, -1.0, 0.5, -0.5, 3.14159, 65504.0, -65504.0}
	for _, f := range tests {
		h := float32ToFloat16(f)
		back := float16ToFloat32(h)
		if back == 0 && f != 0 {
			t.Fatalf("float16 round-trip failed for %f: got 0", f)
		}
	}
}

// TestQuantizeByType 验证按量化类型执行量化编码
func TestQuantizeByType(t *testing.T) {
	vec := []float32{0.5, -0.3, 0.8}

	encFP16, err := quantizeByType(vec, 1, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if encFP16 == nil || len(encFP16) != 6 {
		t.Fatalf("FP16 codec: expected 6 bytes, got %d", len(encFP16))
	}
}

// TestRandomRotation 验证随机旋转矩阵各行归一化
func TestRandomRotation(t *testing.T) {
	dim := 4
	mat := computeRandomRotation(dim, 42)

	if len(mat) != dim {
		t.Fatalf("expected %dx%d matrix", dim, dim)
	}

	for i := 0; i < dim; i++ {
		norm := float32(0)
		for j := 0; j < dim; j++ {
			norm += mat[i][j] * mat[i][j]
		}
		if norm < 0.99 || norm > 1.01 {
			t.Fatalf("row %d not normalized: %f", i, norm)
		}
	}
}
