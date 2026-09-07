package jiracloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

var _ ports.TransitionGateway = (*Session)(nil)

func (s *Session) ListTransitions(ctx context.Context, ref domain.IssueRef) ([]domain.Transition, error) {
	result := []domain.Transition{}
	key, err := domain.IssueKey(ref.Key)
	if err != nil {
		return result, err
	}
	b, err := s.Client.read(ctx, s.Profile, s.Secret, http.MethodGet, "/rest/api/3/issue/"+key+"/transitions?expand=transitions.fields", nil)
	if err != nil {
		return result, err
	}
	var wire struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Category struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
			Fields map[string]struct {
				Name       string `json:"name"`
				Required   bool   `json:"required"`
				HasDefault bool   `json:"hasDefaultValue"`
				Schema     struct {
					Type   string `json:"type"`
					Items  string `json:"items"`
					Custom string `json:"custom"`
				} `json:"schema"`
				Allowed    []json.RawMessage `json:"allowedValues"`
				Operations []string          `json:"operations"`
			} `json:"fields"`
		} `json:"transitions"`
	}
	if json.Unmarshal(b, &wire) != nil || wire.Transitions == nil || len(wire.Transitions) > 200 {
		return result, failure(domain.Unavailable, "Invalid transition metadata from Jira.")
	}
	clean := cleanRemote(s.Profile, s.Secret)
	ids := map[string]bool{}
	for _, t := range wire.Transitions {
		if !domain.ValidWorkflowID(t.ID) || !domain.ValidWorkflowID(t.To.ID) || ids[t.ID] || len(t.Fields) > 200 {
			return result, failure(domain.Unavailable, "Invalid transition metadata from Jira.")
		}
		ids[t.ID] = true
		category, ok := map[string]domain.StatusCategory{"new": domain.CategoryTodo, "indeterminate": domain.CategoryInProgress, "done": domain.CategoryDone}[t.To.Category.Key]
		if !ok {
			category = domain.CategoryUnknown
		}
		transition := domain.Transition{ID: t.ID, Name: clean(t.Name), To: domain.Status{ID: t.To.ID, Name: clean(t.To.Name), Category: category}, Fields: []domain.FieldSpec{}}
		if t.Fields == nil {
			return result, failure(domain.Unavailable, "Jira omitted transition field metadata; no write can be prepared.")
		}
		for id, f := range t.Fields {
			if !domain.ValidFieldID(id) || len(f.Allowed) > 1000 {
				return result, failure(domain.Unavailable, "Invalid or excessive field metadata from Jira.")
			}
			spec := domain.FieldSpec{ID: id, Name: clean(f.Name), Type: f.Schema.Type, ItemType: f.Schema.Items, Required: f.Required, HasDefault: f.HasDefault, Operations: f.Operations, AllowedValues: []domain.NamedID{}}
			if id == "description" || id == "environment" || strings.HasSuffix(f.Schema.Custom, ":textarea") {
				spec.Type = "adf"
			}
			// Jira exposes some date pickers with a string base type.
			if strings.HasSuffix(f.Schema.Custom, ":datepicker") {
				spec.Type = "date"
			}
			if strings.HasSuffix(f.Schema.Custom, ":datetime") {
				spec.Type = "datetime"
			}
			for _, raw := range f.Allowed {
				var value string
				if json.Unmarshal(raw, &value) == nil {
					spec.AllowedValues = append(spec.AllowedValues, domain.NamedID{ID: clean(value), Name: clean(value)})
					continue
				}
				var obj struct {
					ID          string `json:"id"`
					AccountID   string `json:"accountId"`
					Name        string `json:"name"`
					Value       string `json:"value"`
					DisplayName string `json:"displayName"`
				}
				if json.Unmarshal(raw, &obj) != nil {
					return result, failure(domain.Unavailable, "Invalid allowed field value from Jira.")
				}
				optionID := obj.ID
				if optionID == "" {
					optionID = obj.AccountID
				}
				name := obj.Name
				if name == "" {
					name = obj.Value
				}
				if name == "" {
					name = obj.DisplayName
				}
				if optionID == "" {
					return result, failure(domain.Unsupported, "Jira returned field choices without supported IDs; use the browser.")
				}
				spec.AllowedValues = append(spec.AllowedValues, domain.NamedID{ID: clean(optionID), Name: clean(name)})
			}
			transition.Fields = append(transition.Fields, spec)
		}
		sort.Slice(transition.Fields, func(i, j int) bool { return transition.Fields[i].ID < transition.Fields[j].ID })
		result = append(result, transition)
	}
	return result, nil
}

// ApplyTransition invokes HTTP exactly once. It does not use the retrying read
// transport and never adds comments, resolution defaults or other side effects.
func (s *Session) ApplyTransition(ctx context.Context, q domain.TransitionRequest) (domain.ApplyResult, error) {
	result := domain.ApplyResult{State: domain.ApplyFailed, Issue: q.Issue, TransitionID: q.TransitionID}
	key, err := domain.IssueKey(q.Issue.Key)
	if err != nil {
		return result, err
	}
	if !domain.ValidWorkflowID(q.TransitionID) {
		return result, failure(domain.InvalidInput, "Invalid transition ID.")
	}
	if err = s.Profile.Validate(); err != nil {
		return result, err
	}
	if s.Secret.Reveal() == "" {
		return result, failure(domain.Authentication, "Jira credential is missing.")
	}
	for id := range q.Fields {
		if !domain.ValidFieldID(id) || id == "status" || id == "comment" {
			return result, failure(domain.InvalidInput, "Invalid transition field.")
		}
	}
	payload, err := json.Marshal(struct {
		Transition map[string]string            `json:"transition"`
		Fields     map[string]domain.FieldValue `json:"fields,omitempty"`
	}{map[string]string{"id": q.TransitionID}, q.Fields})
	if err != nil || len(payload) > 1024*1024 {
		return result, failure(domain.InvalidInput, "Transition fields must be JSON of at most 1 MiB.")
	}
	if ctx.Err() != nil {
		return result, failure(domain.Canceled, "Operation cancelled before sending.")
	}
	base, _ := s.Profile.BaseURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/rest/api/3/issue/"+key+"/transitions", bytes.NewReader(payload))
	if err != nil {
		return result, failure(domain.InvalidInput, "Could not prepare transition request.")
	}
	req.GetBody = nil
	req.SetBasicAuth(s.Profile.Auth.Email, s.Secret.Reveal())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := *s.Client.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		result.State = domain.ApplyUnknown
		return result, &domain.Error{Kind: domain.Uncertain, Message: "The transition may have reached Jira. It was not retried; inspect the issue before trying again."}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 {
		result.State = domain.ApplyAcceptedUnverified
		return result, nil
	}
	if resp.StatusCode >= 500 || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		result.State = domain.ApplyUnknown
		return result, &domain.Error{Kind: domain.Uncertain, Message: "Jira returned an inconclusive write response. The request was not retried; inspect the issue."}
	}
	kind := domain.Unavailable
	message := "Jira rejected the transition; the request was not retried."
	switch resp.StatusCode {
	case 400, 422:
		kind = domain.Validation
		message = "Jira rejected the transition fields or workflow conditions."
	case 401:
		kind = domain.Authentication
		message = "Jira rejected the credential."
	case 403:
		kind = domain.Forbidden
		message = "Jira denied this transition; check permissions and scopes."
	case 404:
		kind = domain.NotFound
		message = "The issue does not exist or is not visible."
	case 409:
		kind = domain.Conflict
		message = "Jira rejected the transition because of a conflict."
	case 429:
		message = "Jira rate-limited the write. It was not retried."
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		kind = domain.Forbidden
		message = "An authenticated write redirect was rejected."
	}
	public := &domain.Error{Kind: kind, Message: message}
	// Only confirmed validation rejections expose a bounded, redacted field map.
	if kind == domain.Validation {
		b, e := io.ReadAll(io.LimitReader(resp.Body, 65537))
		if e == nil && len(b) <= 65536 {
			var wire struct {
				Errors   map[string]string `json:"errors"`
				Messages []string          `json:"errorMessages"`
			}
			if json.Unmarshal(b, &wire) == nil {
				clean := cleanRemote(s.Profile, s.Secret)
				fields := map[string]string{}
				for id, msg := range wire.Errors {
					if domain.ValidFieldID(id) && len(fields) < 100 {
						fields[id] = truncate(clean(msg), 1024)
					}
				}
				messages := []string{}
				for _, msg := range wire.Messages {
					if len(messages) == 10 {
						break
					}
					messages = append(messages, truncate(clean(msg), 1024))
				}
				public.Details = map[string]any{"fields": fields, "messages": messages}
			}
		}
	}
	return result, public
}
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
