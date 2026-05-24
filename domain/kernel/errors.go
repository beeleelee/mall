package kernel

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrNotFound         ErrorCode = "NOT_FOUND"
	ErrAlreadyExists    ErrorCode = "ALREADY_EXISTS"
	ErrInvalidArgument  ErrorCode = "INVALID_ARGUMENT"
	ErrUnauthenticated  ErrorCode = "UNAUTHENTICATED"
	ErrPermissionDenied ErrorCode = "PERMISSION_DENIED"
	ErrInternal         ErrorCode = "INTERNAL"
	ErrUnavailable      ErrorCode = "UNAVAILABLE"
	ErrConflict         ErrorCode = "CONFLICT"
)

type DomainError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

func NewDomainError(code ErrorCode, msg string) *DomainError {
	return &DomainError{Code: code, Message: msg}
}

func NewDomainErrorWithCause(code ErrorCode, msg string, cause error) *DomainError {
	return &DomainError{Code: code, Message: msg, Err: cause}
}

func IsNotFound(err error) bool {
	return hasCode(err, ErrNotFound)
}

func IsAlreadyExists(err error) bool {
	return hasCode(err, ErrAlreadyExists)
}

func IsInvalidArgument(err error) bool {
	return hasCode(err, ErrInvalidArgument)
}

func IsConflict(err error) bool {
	return hasCode(err, ErrConflict)
}

func IsInternal(err error) bool {
	return hasCode(err, ErrInternal)
}

func IsUnauthenticated(err error) bool {
	return hasCode(err, ErrUnauthenticated)
}

func IsPermissionDenied(err error) bool {
	return hasCode(err, ErrPermissionDenied)
}

func hasCode(err error, code ErrorCode) bool {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}
