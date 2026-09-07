package domain

import (
	"encoding/json"
	"testing"
)

func TestFieldsValidateTypesChoicesAndRequirements(t *testing.T) {
	specs := []FieldSpec{{ID: "summary", Type: "string", Required: true}, {ID: "resolution", Type: "resolution", Required: true, AllowedValues: []NamedID{{ID: "fixed", Name: "Fixed"}}}, {ID: "duedate", Type: "date"}, {ID: "estimate", Type: "number"}, {ID: "assignee", Type: "user"}, {ID: "labels", Type: "array", ItemType: "string"}, {ID: "choices", Type: "array", ItemType: "option", AllowedValues: []NamedID{{ID: "a"}}}}
	values := map[string]any{"summary": "Reviewed", "resolution": map[string]any{"id": "fixed"}, "duedate": "2026-09-08", "estimate": 2.5, "assignee": map[string]any{"accountId": "account-1"}, "labels": []any{"one", "two"}, "choices": []any{map[string]any{"id": "a"}}}
	if err := ValidateFields(specs, values, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id    string
		value any
	}{{"summary", ""}, {"resolution", map[string]any{"id": "cancelled"}}, {"duedate", "2026-02-30"}, {"estimate", "large"}, {"assignee", map[string]any{"name": "admin"}}, {"choices", []any{map[string]any{"id": "unknown"}}}, {"status", map[string]any{"id": "done"}}, {"comment", "hidden side effect"}} {
		copy := map[string]any{}
		for k, v := range values {
			copy[k] = v
		}
		copy[tc.id] = tc.value
		if e := ValidateFields(specs, copy, true); e == nil {
			t.Fatal("accepted", tc.id)
		}
	}
	if e := ValidateFields(specs, map[string]any{}, true); ExitCode(e) != 6 {
		t.Fatal(e)
	}
	if e := ValidateFields([]FieldSpec{{ID: "plugin", Type: "opaque"}}, map[string]any{"plugin": map[string]any{}}, true); ExitCode(e) != 11 {
		t.Fatal(e)
	}
	if e := ValidateFields([]FieldSpec{{ID: "labels", Type: "array", ItemType: "string", Operations: []string{"add"}}}, map[string]any{"labels": []any{"x"}}, true); ExitCode(e) != 11 {
		t.Fatal(e)
	}
	if e := ValidateFields([]FieldSpec{{ID: "serverDefault", Type: "string", Required: true, HasDefault: true}}, map[string]any{}, true); e != nil {
		t.Fatal(e)
	}
}
func TestFieldVerificationUsesActualIDsAndArrayValues(t *testing.T) {
	want := map[string]any{"resolution": map[string]any{"id": "fixed"}, "assignee": map[string]any{"accountId": "u"}, "labels": []any{"a", "b"}, "number": json.Number("2")}
	got := map[string]any{"resolution": map[string]any{"id": "fixed", "name": "Fixed", "self": "https://example"}, "assignee": map[string]any{"accountId": "u", "displayName": "User"}, "labels": []any{"b", "a"}, "number": float64(2)}
	if !FieldsMatch(want, got) {
		t.Fatal("matching IDs rejected")
	}
	got["resolution"] = map[string]any{"id": "cancelled"}
	if FieldsMatch(want, got) {
		t.Fatal("wrong resolution verified")
	}
}

func TestPlainTextADFValidation(t *testing.T) {
	specs := []FieldSpec{{ID: "description", Type: "adf"}}
	if err := ValidateFields(specs, map[string]any{"description": TextADF("Completed review")}, true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFields(specs, map[string]any{"description": "raw string"}, true); err == nil {
		t.Fatal("accepted a string for an ADF field")
	}
}

func TestADFVerificationPreservesParagraphOrderAndRequiredText(t *testing.T) {
	if FieldsMatch(map[string]any{"description": TextADF("first\nsecond")}, map[string]any{"description": TextADF("second\nfirst")}) {
		t.Fatal("reordered content verified")
	}
	if !FieldsMatch(map[string]any{"description": TextADF("first\nsecond")}, map[string]any{"description": TextADF("first\nsecond")}) {
		t.Fatal("same content rejected")
	}
	if err := ValidateFields([]FieldSpec{{ID: "description", Type: "adf", Required: true}}, map[string]any{"description": TextADF(" ")}, true); err == nil {
		t.Fatal("blank required text accepted")
	}
}
