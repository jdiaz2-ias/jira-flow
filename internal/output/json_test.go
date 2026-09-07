package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/domain"
)

func TestFailureHidesUnderlyingCause(t *testing.T) {
	err := &domain.Error{Kind: domain.Authentication, Message: "Credencial rechazada.", Cause: errors.New("synthetic-private-token")}
	var out bytes.Buffer
	if err := Write(&out, Failure(err)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "synthetic-private-token") {
		t.Fatal("cause exposed")
	}
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["data"] != nil || envelope["ok"] != false {
		t.Fatalf("invalid failure: %v", envelope)
	}
	if _, ok := envelope["warnings"].([]any); !ok {
		t.Fatal("warnings must be an array")
	}
}

func TestSerializationFailureWritesNothing(t *testing.T) {
	var out bytes.Buffer
	if err := Write(&out, Success(make(chan int))); err == nil {
		t.Fatal("expected serialization failure")
	}
	if out.Len() != 0 {
		t.Fatal("partial JSON written")
	}
}
