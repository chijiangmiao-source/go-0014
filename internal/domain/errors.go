package domain

import "fmt"

type ErrorCode string

const (
	CodeSampleConflict      ErrorCode = "SAMPLE_CONFLICT"
	CodeVersionConflict     ErrorCode = "VERSION_CONFLICT"
	CodeIllegalState        ErrorCode = "ILLEGAL_STATE"
	CodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"
	CodeDataUnrecoverable   ErrorCode = "DATA_UNRECOVERABLE"
	CodeEvidenceCorrupt     ErrorCode = "EVIDENCE_CORRUPT"
	CodeInvalidInput        ErrorCode = "INVALID_INPUT"
	CodeComputeTemporary    ErrorCode = "COMPUTE_TEMPORARY"
)

type BoundaryError struct {
	Code      ErrorCode `json:"code"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"`
	Details   []string  `json:"details,omitempty"`
}

func (e *BoundaryError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewInputError(code ErrorCode, message string, details ...string) *BoundaryError {
	return &BoundaryError{Code: code, Category: "input", Message: message, Details: details}
}

func NewStateError(code ErrorCode, message string) *BoundaryError {
	return &BoundaryError{Code: code, Category: "state", Message: message}
}

func NewTemporaryError(message string, details ...string) *BoundaryError {
	return &BoundaryError{Code: CodeComputeTemporary, Category: "compute", Message: message, Retryable: true, Details: details}
}
