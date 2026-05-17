package apperrors

import (
	"errors"
	"fmt"
)

const (
	errorWithOperationFormat = "%s: %s"
)

type Kind string

const (
	KindValidation      Kind = "validation"
	KindNotFound        Kind = "not_found"
	KindPermission      Kind = "permission_denied"
	KindExternalService Kind = "external_service"
	KindRateLimited     Kind = "rate_limited"
	KindInternal        Kind = "internal"
)

type Error struct {
	Kind    Kind
	Message string
	Op      string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Message
	}
	return fmt.Sprintf(errorWithOperationFormat, e.Op, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(kind Kind, message string) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
	}
}

func Wrap(kind Kind, op, message string, err error) *Error {
	return &Error{
		Kind:    kind,
		Op:      op,
		Message: message,
		Err:     err,
	}
}

func Validation(message string) *Error {
	return New(KindValidation, message)
}

func NotFound(message string) *Error {
	return New(KindNotFound, message)
}

func PermissionDenied(message string) *Error {
	return New(KindPermission, message)
}

func ExternalService(message string) *Error {
	return New(KindExternalService, message)
}

func RateLimited(message string) *Error {
	return New(KindRateLimited, message)
}

func Internal(message string) *Error {
	return New(KindInternal, message)
}

func IsKind(err error, kind Kind) bool {
	var appErr *Error
	return errors.As(err, &appErr) && appErr.Kind == kind
}

func IsNotFound(err error) bool {
	return IsKind(err, KindNotFound)
}
