package domain

import "errors"

type ErrorKind string

const (
	Internal            ErrorKind = "internal_error"
	InvalidInput        ErrorKind = "invalid_input"
	Authentication      ErrorKind = "authentication_required"
	Forbidden           ErrorKind = "forbidden"
	NotFound            ErrorKind = "not_found"
	Validation          ErrorKind = "validation_failed"
	TransitionAmbiguous ErrorKind = "transition_ambiguous"
	Conflict            ErrorKind = "conflict"
	Unavailable         ErrorKind = "service_unavailable"
	Uncertain           ErrorKind = "write_uncertain"
	Partial             ErrorKind = "partial_result"
	Unsupported         ErrorKind = "capability_unavailable"
	Canceled            ErrorKind = "canceled"
)

// Message must be safe for users. Cause is retained for classification, not output.
type Error struct {
	Kind      ErrorKind
	Message   string
	Retryable bool
	Cause     error
	Details   map[string]any
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if !errors.As(err, &e) {
		return 1
	}
	switch e.Kind {
	case InvalidInput:
		return 2
	case Authentication:
		return 3
	case Forbidden:
		return 4
	case NotFound:
		return 5
	case Validation, TransitionAmbiguous:
		return 6
	case Conflict:
		return 7
	case Unavailable:
		return 8
	case Uncertain:
		return 9
	case Partial:
		return 10
	case Unsupported:
		return 11
	case Canceled:
		return 130
	default:
		return 1
	}
}
