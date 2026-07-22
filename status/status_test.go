package status

import "testing"

// TestOKStatus 验证 OK 状态码、消息和 GoError 为 nil
func TestOKStatus(t *testing.T) {
	s := OKStatus()
	if !s.OK() {
		t.Fatal("expected OK")
	}
	if s.Code() != OK {
		t.Fatalf("expected code OK, got %d", s.Code())
	}
	if s.Error() != "OK" {
		t.Fatalf("expected 'OK', got '%s'", s.Error())
	}
	if s.GoError() != nil {
		t.Fatal("expected nil GoError for OK status")
	}
}

// TestNotFound 验证 NotFound 状态码、消息和 GoError 格式
func TestNotFound(t *testing.T) {
	s := NewNotFound("doc not found")
	if s.OK() {
		t.Fatal("expected not OK")
	}
	if s.Code() != NotFound {
		t.Fatalf("expected NotFound, got %d", s.Code())
	}
	if s.Message() != "doc not found" {
		t.Fatalf("expected 'doc not found', got '%s'", s.Message())
	}
	err := s.GoError()
	if err == nil {
		t.Fatal("expected non-nil GoError")
	}
	if err.Error() != "NOT_FOUND: doc not found" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
}

// TestInvalidArgument 验证 InvalidArgument 状态码
func TestInvalidArgument(t *testing.T) {
	s := NewInvalidArgument("bad input")
	if s.Code() != InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %d", s.Code())
	}
}

// TestAlreadyExists 验证 AlreadyExists 状态码
func TestAlreadyExists(t *testing.T) {
	s := NewAlreadyExists("dup")
	if s.Code() != AlreadyExists {
		t.Fatal("expected AlreadyExists")
	}
}

// TestInternalError 验证 InternalError 状态码
func TestInternalError(t *testing.T) {
	s := NewInternalError("crash")
	if s.Code() != InternalError {
		t.Fatal("expected InternalError")
	}
}

// TestCodeString 验证状态码字符串表示
func TestCodeString(t *testing.T) {
	tests := []struct {
		code Code
		want string
	}{
		{OK, "OK"},
		{NotFound, "NOT_FOUND"},
		{AlreadyExists, "ALREADY_EXISTS"},
		{InvalidArgument, "INVALID_ARGUMENT"},
		{InternalError, "INTERNAL_ERROR"},
		{Code(999), "CODE(999)"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("Code(%d).String() = %s, want %s", tt.code, got, tt.want)
		}
	}
}

// TestResultOk 验证 Result 成功状态的值和 Unwrap
func TestResultOk(t *testing.T) {
	r := Ok(42)
	if !r.OK() {
		t.Fatal("expected OK result")
	}
	if r.Value() != 42 {
		t.Fatalf("expected 42, got %d", r.Value())
	}
	v, err := r.Unwrap()
	if v != 42 || err != nil {
		t.Fatalf("expected (42, nil), got (%d, %v)", v, err)
	}
}

// TestResultErr 验证 Result 错误状态的状态码和 Unwrap
func TestResultErr(t *testing.T) {
	r := Err[int](NewNotFound("missing"))
	if r.OK() {
		t.Fatal("expected not OK result")
	}
	if r.Status().Code() != NotFound {
		t.Fatal("expected NotFound status")
	}
	_, err := r.Unwrap()
	if err == nil {
		t.Fatal("expected non-nil error from Unwrap")
	}
}
