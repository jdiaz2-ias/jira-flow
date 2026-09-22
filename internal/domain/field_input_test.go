package domain

import "testing"

func TestInteractiveFieldConversionUsesWriteValidation(t *testing.T) {
	for _, tc := range []struct {
		kind, input string
		valid       bool
	}{
		{"string", "text", true}, {"date", "2026-09-21", true}, {"date", "yesterday", false},
		{"integer", "1.5", false}, {"integer", "2", true}, {"number", "NaN", false},
		{"boolean", "true", true}, {"boolean", "maybe", false}, {"user", "account", true},
		{"adf", "paragraph", true}, {"adf", "", false}, {"unsupported", "value", false},
	} {
		_, err := ParseFieldInput(FieldSpec{ID: "customfield_1", Type: tc.kind, Required: true}, tc.input)
		if (err == nil) != tc.valid {
			t.Fatalf("%s %q: %v", tc.kind, tc.input, err)
		}
	}
	spec := FieldSpec{ID: "resolution", Type: "resolution", Required: true, AllowedValues: []NamedID{{ID: "fixed"}}}
	if _, err := ParseFieldInput(spec, "unknown"); err == nil {
		t.Fatal("invalid option accepted")
	}
	if _, err := ParseFieldInput(spec, "fixed"); err != nil {
		t.Fatal(err)
	}
	spec = FieldSpec{ID: "customfield_2", Type: "array", ItemType: "option", Required: true, AllowedValues: []NamedID{{ID: "a"}}}
	if _, err := ParseFieldInput(spec, `[{"id":"a"}]`); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFieldInput(spec, `[{"id":"b"}]`); err == nil {
		t.Fatal("invalid array accepted")
	}
}
