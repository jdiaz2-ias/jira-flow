package domain

import (
	"encoding/json"
	"strconv"
)

// ParseFieldInput converts interactive text using the same schema checks as writes.
func ParseFieldInput(f FieldSpec, answer string) (any, error) {
	var value any
	switch f.Type {
	case "adf":
		value = TextADF(answer)
	case "string", "date", "datetime":
		value = answer
	case "number", "integer":
		n, e := strconv.ParseFloat(answer, 64)
		if e != nil {
			return nil, &Error{Kind: Validation, Message: "Enter a valid number."}
		}
		value = n
	case "boolean":
		v, e := strconv.ParseBool(answer)
		if e != nil {
			return nil, &Error{Kind: Validation, Message: "Enter true or false."}
		}
		value = v
	case "user":
		value = map[string]any{"accountId": answer}
	case "option", "resolution", "priority", "issuetype", "project", "version", "component":
		value = map[string]any{"id": answer}
	case "array":
		if json.Unmarshal([]byte(answer), &value) != nil {
			return nil, &Error{Kind: Validation, Message: "Enter a JSON array of values or ID objects."}
		}
	default:
		return nil, &Error{Kind: Unsupported, Message: "Unsupported field schema. Use jflow open to complete this transition in Jira."}
	}
	if err := ValidateFields([]FieldSpec{f}, map[string]any{f.ID: value}, true); err != nil {
		return nil, err
	}
	return value, nil
}
