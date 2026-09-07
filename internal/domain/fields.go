package domain

import (
	"encoding/json"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var fieldIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,127}$`)
var workflowIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func ValidFieldID(s string) bool    { return fieldIDPattern.MatchString(s) }
func ValidWorkflowID(s string) bool { return workflowIDPattern.MatchString(s) }

type WorkflowRule struct {
	ProjectID          string `json:"project_id"`
	IssueTypeID        string `json:"issue_type_id"`
	Intent             Intent `json:"intent"`
	FromStatusID       string `json:"from_status_id"`
	TransitionID       string `json:"transition_id"`
	ExpectedToStatusID string `json:"expected_to_status_id"`
}

func (r WorkflowRule) Validate() error {
	if r.Intent != IntentStart && r.Intent != IntentDone && r.Intent != IntentClose && r.Intent != IntentReopen {
		return &Error{Kind: InvalidInput, Message: "Invalid workflow intent."}
	}
	for _, id := range []string{r.ProjectID, r.IssueTypeID, r.FromStatusID, r.TransitionID, r.ExpectedToStatusID} {
		if !ValidWorkflowID(id) {
			return &Error{Kind: InvalidInput, Message: "Workflow rules require real project, issue type, state and transition IDs."}
		}
	}
	return nil
}
func MissingFields(specs []FieldSpec, values map[string]FieldValue) []FieldSpec {
	missing := []FieldSpec{}
	for _, s := range specs {
		v, ok := values[s.ID]
		if s.Required && !s.HasDefault && (!ok || v == nil || v == "" || emptyArray(v)) {
			missing = append(missing, s)
		}
	}
	return missing
}
func emptyArray(v any) bool { a, ok := v.([]any); return ok && len(a) == 0 }
func fieldError(kind ErrorKind, id, msg string) error {
	return &Error{Kind: kind, Message: msg, Details: map[string]any{"fields": map[string]string{id: msg}}}
}

// ValidateFields rejects unknown keys and values outside metadata. Missing fields
// can be deferred while an interactive form is being assembled.
func ValidateFields(specs []FieldSpec, values map[string]FieldValue, requireAll bool) error {
	known := map[string]FieldSpec{}
	for _, s := range specs {
		known[s.ID] = s
	}
	keys := []string{}
	for id := range values {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		s, ok := known[id]
		if !ok || !ValidFieldID(id) || id == "status" || id == "comment" {
			return fieldError(Validation, id, "Field is not writable through this transition.")
		}
		if len(s.Operations) > 0 {
			canSet := false
			for _, op := range s.Operations {
				if op == "set" {
					canSet = true
				}
			}
			if !canSet {
				return fieldError(Unsupported, id, "This field does not support direct set operations; use Jira in the browser.")
			}
		}
		v := values[id]
		if v == nil {
			if s.Required {
				return fieldError(Validation, id, "A required field cannot be null.")
			}
			continue
		}
		if err := validateValue(s, v); err != nil {
			return err
		}
	}
	if requireAll {
		missing := MissingFields(specs, values)
		if len(missing) > 0 {
			details := map[string]string{}
			for _, f := range missing {
				details[f.ID] = "A required field is missing."
			}
			return &Error{Kind: Validation, Message: "Required transition fields are missing.", Details: map[string]any{"fields": details}}
		}
	}
	return nil
}
func validateValue(s FieldSpec, v any) error {
	invalid := func() error {
		return fieldError(Validation, s.ID, "Field value does not match its schema or allowed values.")
	}
	switch s.Type {
	case "string":
		value, ok := v.(string)
		if !ok || (s.Required && strings.TrimSpace(value) == "") {
			return invalid()
		}
		if len(s.AllowedValues) > 0 && !allowed(s, value) {
			return invalid()
		}
	case "number", "integer":
		var n float64
		switch value := v.(type) {
		case float64:
			n = value
		case json.Number:
			var e error
			n, e = value.Float64()
			if e != nil {
				return invalid()
			}
		default:
			return invalid()
		}
		if math.IsNaN(n) || math.IsInf(n, 0) || (s.Type == "integer" && math.Trunc(n) != n) {
			return invalid()
		}
	case "date", "datetime":
		value, ok := v.(string)
		if !ok {
			return invalid()
		}
		layout := "2006-01-02"
		if s.Type == "datetime" {
			layout = time.RFC3339
		}
		if _, e := time.Parse(layout, value); e != nil {
			return invalid()
		}
	case "adf":
		if !validTextADF(v) || (s.Required && !hasADFText(v)) {
			return invalid()
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return invalid()
		}
	case "user":
		obj, ok := v.(map[string]any)
		if !ok || len(obj) != 1 {
			return invalid()
		}
		id, ok := obj["accountId"].(string)
		if !ok || id == "" || (len(s.AllowedValues) > 0 && !allowed(s, id)) {
			return invalid()
		}
	case "option", "resolution", "priority", "issuetype", "project", "version", "component":
		obj, ok := v.(map[string]any)
		if !ok || len(obj) != 1 {
			return invalid()
		}
		id, ok := obj["id"].(string)
		if !ok || id == "" || (len(s.AllowedValues) > 0 && !allowed(s, id)) {
			return invalid()
		}
	case "array":
		list, ok := v.([]any)
		if !ok || len(list) > 1000 || (s.Required && len(list) == 0) {
			return invalid()
		}
		child := s
		child.Type = s.ItemType
		child.Required = false
		for _, item := range list {
			if item == nil {
				return invalid()
			}
			if err := validateValue(child, item); err != nil {
				return err
			}
		}
	default:
		return fieldError(Unsupported, s.ID, "This field schema is not supported; open the issue in Jira. No change was sent.")
	}
	return nil
}
func allowed(s FieldSpec, id string) bool {
	for _, option := range s.AllowedValues {
		if option.ID == id {
			return true
		}
	}
	return false
}

// FieldsMatch compares submitted IDs/scalars while ignoring server-added object
// properties. Multi-value fields are compared as multisets, not by display order.
func FieldsMatch(expected, observed map[string]FieldValue) bool {
	for key, want := range expected {
		got, ok := observed[key]
		if !ok || !valueMatches(want, got) {
			return false
		}
	}
	return true
}
func valueMatches(want, got any) bool {
	if obj, ok := want.(map[string]any); ok {
		actual, ok := got.(map[string]any)
		if !ok {
			return false
		}
		if obj["type"] == "doc" {
			return orderedValueMatches(obj, actual)
		}
		return FieldsMatch(obj, actual)
	}
	if list, ok := want.([]any); ok {
		actual, ok := got.([]any)
		if !ok || len(list) != len(actual) {
			return false
		}
		used := make([]bool, len(actual))
		for _, v := range list {
			found := false
			for i, a := range actual {
				if !used[i] && valueMatches(v, a) {
					used[i] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	// JSON normalization makes json.Number and decoded float64 comparable.
	a, e1 := json.Marshal(want)
	b, e2 := json.Marshal(got)
	if e1 == nil && e2 == nil && string(a) == string(b) {
		return true
	}
	return reflect.DeepEqual(want, got)
}

// TextADF represents plain text as Jira Cloud paragraph/text nodes. Markdown
// syntax remains literal; this is not a Markdown-to-ADF conversion.
func TextADF(text string) map[string]any {
	content := []any{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		nodes := []any{}
		if line != "" {
			nodes = append(nodes, map[string]any{"type": "text", "text": line})
		}
		content = append(content, map[string]any{"type": "paragraph", "content": nodes})
	}
	return map[string]any{"type": "doc", "version": float64(1), "content": content}
}
func validTextADF(value any) bool {
	root, ok := value.(map[string]any)
	if !ok || len(root) != 3 || root["type"] != "doc" || root["version"] != float64(1) {
		return false
	}
	paragraphs, ok := root["content"].([]any)
	if !ok || len(paragraphs) > 10000 {
		return false
	}
	nodes := 0
	for _, raw := range paragraphs {
		p, ok := raw.(map[string]any)
		if !ok || len(p) != 2 || p["type"] != "paragraph" {
			return false
		}
		content, ok := p["content"].([]any)
		if !ok {
			return false
		}
		for _, rawNode := range content {
			nodes++
			if nodes > 10000 {
				return false
			}
			n, ok := rawNode.(map[string]any)
			if !ok || len(n) != 2 || n["type"] != "text" {
				return false
			}
			if _, ok := n["text"].(string); !ok {
				return false
			}
		}
	}
	return true
}

// ADF content order changes its meaning, unlike multi-select field ordering.
func orderedValueMatches(want, got any) bool {
	switch v := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for key, child := range v {
			other, exists := actual[key]
			if !exists || !orderedValueMatches(child, other) {
				return false
			}
		}
		return true
	case []any:
		actual, ok := got.([]any)
		if !ok || len(v) != len(actual) {
			return false
		}
		for i, child := range v {
			if !orderedValueMatches(child, actual[i]) {
				return false
			}
		}
		return true
	default:
		return valueMatches(want, got)
	}
}
func hasADFText(v any) bool {
	root, ok := v.(map[string]any)
	if !ok {
		return false
	}
	paragraphs, _ := root["content"].([]any)
	for _, raw := range paragraphs {
		paragraph, _ := raw.(map[string]any)
		content, _ := paragraph["content"].([]any)
		for _, rawNode := range content {
			node, _ := rawNode.(map[string]any)
			text, _ := node["text"].(string)
			if strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}
