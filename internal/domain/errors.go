package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors
var (
	ErrNotFound             = errors.New("memory not found")
	ErrDuplicate            = errors.New("duplicate memory")
	ErrValidation           = errors.New("validation error")
	ErrConflict             = errors.New("conflict")
	ErrStorageUnavailable   = errors.New("storage unavailable")
	ErrEmbeddingUnavailable = errors.New("embedding provider unavailable")
)

// NotFoundError carries the missing ID
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("memory not found: %s", e.ID) }
func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

// ValidationError carries a single field error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message) }
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

// ValidationErrors is a collection of field errors
type ValidationErrors struct {
	Errors []*ValidationError
}

func (e *ValidationErrors) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

func (e *ValidationErrors) Is(target error) bool { return target == ErrValidation }

func (e *ValidationErrors) Add(field, message string) {
	e.Errors = append(e.Errors, &ValidationError{Field: field, Message: message})
}

func (e *ValidationErrors) HasErrors() bool { return len(e.Errors) > 0 }

// DuplicateError carries the existing memory ID
type DuplicateError struct {
	ExistingID     string
	SimilarityType string // "exact", "fuzzy", "semantic"
	Score          float64
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("duplicate memory: existing=%s type=%s score=%.3f", e.ExistingID, e.SimilarityType, e.Score)
}
func (e *DuplicateError) Is(target error) bool { return target == ErrDuplicate }
