// Package domain provides core domain types and business logic.
package domain

import (
	"errors"
	"fmt"
)

// ErrorCode represents a category of domain errors
type ErrorCode string

const (
	// ErrCodeNotFound indicates a resource was not found
	ErrCodeNotFound ErrorCode = "NOT_FOUND"
	// ErrCodeInvalidInput indicates invalid input was provided
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	// ErrCodeInternal indicates an internal server error
	ErrCodeInternal ErrorCode = "INTERNAL_ERROR"
	// ErrCodeUnavailable indicates a resource is unavailable
	ErrCodeUnavailable ErrorCode = "UNAVAILABLE"
	// ErrCodeTimeout indicates an operation timed out
	ErrCodeTimeout ErrorCode = "TIMEOUT"
	// ErrCodeConflict indicates a conflict state
	ErrCodeConflict ErrorCode = "CONFLICT"
)

// DomainError represents a structured domain error with context
type DomainError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *DomainError) WithContext(ctx map[string]interface{}) *DomainError {
	e.Context = ctx
	return e
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeNotFound,
		Message: message,
		Cause:   cause,
	}
}

// NewInvalidInputError creates an invalid input error
func NewInvalidInputError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeInvalidInput,
		Message: message,
		Cause:   cause,
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeInternal,
		Message: message,
		Cause:   cause,
	}
}

// NewUnavailableError creates an unavailable error
func NewUnavailableError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeUnavailable,
		Message: message,
		Cause:   cause,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeTimeout,
		Message: message,
		Cause:   cause,
	}
}

// NewConflictError creates a conflict error
func NewConflictError(message string, cause error) *DomainError {
	return &DomainError{
		Code:    ErrCodeConflict,
		Message: message,
		Cause:   cause,
	}
}

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeNotFound
	}
	return false
}

// IsInvalidInput checks if error is an invalid input error
func IsInvalidInput(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeInvalidInput
	}
	return false
}

// IsInternal checks if error is an internal error
func IsInternal(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeInternal
	}
	return false
}

// IsUnavailable checks if error is an unavailable error
func IsUnavailable(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeUnavailable
	}
	return false
}

// IsTimeout checks if error is a timeout error
func IsTimeout(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeTimeout
	}
	return false
}

// IsConflict checks if error is a conflict error
func IsConflict(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrCodeConflict
	}
	return false
}

// GetDomainError extracts the DomainError from an error if it exists
func GetDomainError(err error) *DomainError {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr
	}
	return nil
}
