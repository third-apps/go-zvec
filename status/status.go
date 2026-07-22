package status

import "fmt"

type Code uint32

const (
	OK                 Code = 0
	NotFound           Code = 1
	AlreadyExists      Code = 2
	InvalidArgument    Code = 3
	PermissionDenied   Code = 4
	FailedPrecondition Code = 5
	ResourceExhausted  Code = 6
	Unavailable        Code = 7
	InternalError      Code = 8
	NotSupported       Code = 9
	Unknown            Code = 10
)

func (c Code) String() string {
	switch c {
	case OK:
		return "OK"
	case NotFound:
		return "NOT_FOUND"
	case AlreadyExists:
		return "ALREADY_EXISTS"
	case InvalidArgument:
		return "INVALID_ARGUMENT"
	case PermissionDenied:
		return "PERMISSION_DENIED"
	case FailedPrecondition:
		return "FAILED_PRECONDITION"
	case ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case Unavailable:
		return "UNAVAILABLE"
	case InternalError:
		return "INTERNAL_ERROR"
	case NotSupported:
		return "NOT_SUPPORTED"
	case Unknown:
		return "UNKNOWN"
	default:
		return fmt.Sprintf("CODE(%d)", c)
	}
}

type Status struct {
	code    Code
	message string
}

func (s Status) OK() bool {
	return s.code == OK
}

func (s Status) Code() Code {
	return s.code
}

func (s Status) Message() string {
	return s.message
}

func (s Status) Error() string {
	if s.OK() {
		return "OK"
	}
	return fmt.Sprintf("%s: %s", s.code, s.message)
}

func (s Status) GoError() error {
	if s.OK() {
		return nil
	}
	return &statusError{Status: s}
}

func (s Status) GoErrorWithCause(cause error) error {
	if s.OK() {
		return nil
	}
	return &statusError{Status: s, cause: cause}
}

type statusError struct {
	cause error
	Status
}

func (e *statusError) Unwrap() error {
	return e.cause
}

func (e *statusError) Error() string {
	if e.Status.OK() {
		return "OK"
	}
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Status.code, e.Status.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Status.code, e.Status.message)
}

func OKStatus() Status {
	return Status{code: OK}
}

func NewNotFound(msg string) Status {
	return Status{code: NotFound, message: msg}
}

func NewAlreadyExists(msg string) Status {
	return Status{code: AlreadyExists, message: msg}
}

func NewInvalidArgument(msg string) Status {
	return Status{code: InvalidArgument, message: msg}
}

func NewPermissionDenied(msg string) Status {
	return Status{code: PermissionDenied, message: msg}
}

func NewFailedPrecondition(msg string) Status {
	return Status{code: FailedPrecondition, message: msg}
}

func NewResourceExhausted(msg string) Status {
	return Status{code: ResourceExhausted, message: msg}
}

func NewUnavailable(msg string) Status {
	return Status{code: Unavailable, message: msg}
}

func NewInternalError(msg string) Status {
	return Status{code: InternalError, message: msg}
}

func NewNotSupported(msg string) Status {
	return Status{code: NotSupported, message: msg}
}

func NewUnknown(msg string) Status {
	return Status{code: Unknown, message: msg}
}

type Result[T any] struct {
	value  T
	status Status
}

func Ok[T any](v T) Result[T] {
	return Result[T]{value: v, status: OKStatus()}
}

func Err[T any](s Status) Result[T] {
	return Result[T]{status: s}
}

func (r Result[T]) OK() bool {
	return r.status.OK()
}

func (r Result[T]) Value() T {
	return r.value
}

func (r Result[T]) Status() Status {
	return r.status
}

func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.status.GoError()
}
