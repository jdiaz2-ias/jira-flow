package domain

import "time"

type Intent string

const (
	IntentStart  Intent = "start"
	IntentDone   Intent = "done"
	IntentClose  Intent = "close"
	IntentReopen Intent = "reopen"
)

type FieldSpec struct {
	ID, Name, Type, ItemType string
	Required                 bool
	AllowedValues            []NamedID
}

// FieldValue is a JSON-compatible value validated by the workflow adapter.
type FieldValue = any

type Transition struct {
	ID, Name string
	To       Status
	Fields   []FieldSpec
}

type TransitionRequest struct {
	Issue        IssueRef
	TransitionID string
	Fields       map[string]FieldValue
}

type PreparedAction struct {
	Profile        string
	Issue          IssueRef
	Intent         Intent
	Transition     Transition
	Fields         map[string]FieldValue
	ObservedStatus Status
	ObservedAt     time.Time
}

type ApplyState string

const (
	ApplyVerified           ApplyState = "verified"
	ApplyAcceptedUnverified ApplyState = "accepted_unverified"
	ApplyUnknown            ApplyState = "unknown"
	ApplyFailed             ApplyState = "failed"
	ApplyNoop               ApplyState = "noop"
)

type ApplyResult struct {
	State            ApplyState
	Issue            IssueRef
	TransitionID     string
	Before, Observed *Status
}
