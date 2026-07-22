package persist

import (
	"bytes"
	"testing"
)

// TestHeaderRoundTrip 验证文件头序列化与反序列化一致性
func TestHeaderRoundTrip(t *testing.T) {
	expected := FileHeader{Magic: MagicNumber, Version: 1, IndexType: IndexTypeHNSW}
	var buf bytes.Buffer
	if err := WriteHeader(&buf, expected); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHeader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Magic != expected.Magic || got.Version != expected.Version || got.IndexType != expected.IndexType {
		t.Fatalf("header mismatch: got %+v, want %+v", got, expected)
	}
}

// TestStringRoundTrip 验证字符串（含 Unicode）序列化与反序列化一致性
func TestStringRoundTrip(t *testing.T) {
	s := "hello世界"
	var buf bytes.Buffer
	if err := WriteString(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadString(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != s {
		t.Fatalf("string mismatch: got %q, want %q", got, s)
	}
}

// TestFloat32SliceRoundTrip 验证 float32 切片序列化与反序列化一致性
func TestFloat32SliceRoundTrip(t *testing.T) {
	s := []float32{1.0, 2.5, -3.14, 0}
	var buf bytes.Buffer
	if err := WriteFloat32Slice(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFloat32Slice(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(s) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(s))
	}
	for i := range s {
		if got[i] != s[i] {
			t.Fatalf("slice mismatch at %d: got %f, want %f", i, got[i], s[i])
		}
	}
}

// TestBoolSliceRoundTrip 验证布尔切片序列化与反序列化一致性
func TestBoolSliceRoundTrip(t *testing.T) {
	s := []bool{true, false, true, true, false, true, false, false, true}
	var buf bytes.Buffer
	if err := WriteBoolSlice(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBoolSlice(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(s) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(s))
	}
	for i := range s {
		if got[i] != s[i] {
			t.Fatalf("bool slice mismatch at %d: got %v, want %v", i, got[i], s[i])
		}
	}
}

// TestIntSliceRoundTrip 验证 int 切片序列化与反序列化一致性
func TestIntSliceRoundTrip(t *testing.T) {
	s := []int{0, 1, -1, 42, 1000}
	var buf bytes.Buffer
	if err := WriteIntSlice(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIntSlice(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(s) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(s))
	}
	for i := range s {
		if got[i] != s[i] {
			t.Fatalf("int slice mismatch at %d: got %d, want %d", i, got[i], s[i])
		}
	}
}

// TestUint64SliceRoundTrip 验证 uint64 切片序列化与反序列化一致性
func TestUint64SliceRoundTrip(t *testing.T) {
	s := []uint64{0, 1, 42, 18446744073709551615}
	var buf bytes.Buffer
	if err := WriteUint64Slice(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadUint64Slice(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(s) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(s))
	}
	for i := range s {
		if got[i] != s[i] {
			t.Fatalf("uint64 slice mismatch at %d: got %d, want %d", i, got[i], s[i])
		}
	}
}

// TestEmptySlices 验证空切片和空字符串序列化与反序列化一致性
func TestEmptySlices(t *testing.T) {
	var buf bytes.Buffer

	if err := WriteFloat32Slice(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteBoolSlice(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteIntSlice(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteString(&buf, ""); err != nil {
		t.Fatal(err)
	}

	gotF, err := ReadFloat32Slice(&buf)
	if err != nil || len(gotF) != 0 {
		t.Fatalf("empty float32 slice: got %v, err %v", gotF, err)
	}
	gotB, err := ReadBoolSlice(&buf)
	if err != nil || len(gotB) != 0 {
		t.Fatalf("empty bool slice: got %v, err %v", gotB, err)
	}
	gotI, err := ReadIntSlice(&buf)
	if err != nil || len(gotI) != 0 {
		t.Fatalf("empty int slice: got %v, err %v", gotI, err)
	}
	gotS, err := ReadString(&buf)
	if err != nil || gotS != "" {
		t.Fatalf("empty string: got %q, err %v", gotS, err)
	}
}
