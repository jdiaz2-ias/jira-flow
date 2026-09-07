package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type TransitionField struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	ItemType   string   `json:"item_type,omitempty"`
	Required   bool     `json:"required"`
	HasDefault bool     `json:"has_default"`
	Allowed    []Named  `json:"allowed_values"`
	Operations []string `json:"operations"`
}
type TransitionData struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	To     IssueStatus       `json:"to"`
	Fields []TransitionField `json:"fields"`
}

func status(s domain.Status) IssueStatus { return IssueStatus{s.ID, s.Name, s.Category} }
func transition(t domain.Transition) TransitionData {
	out := TransitionData{ID: t.ID, Name: t.Name, To: status(t.To), Fields: []TransitionField{}}
	for _, f := range t.Fields {
		allowed := []Named{}
		for _, v := range f.AllowedValues {
			allowed = append(allowed, Named{v.ID, v.Name})
		}
		ops := append([]string{}, f.Operations...)
		out.Fields = append(out.Fields, TransitionField{f.ID, f.Name, f.Type, f.ItemType, f.Required, f.HasDefault, allowed, ops})
	}
	return out
}
func TransitionsEnvelope(profile string, c app.Catalog, err error) Envelope {
	e := Success(nil)
	if err != nil {
		e = Failure(err)
	}
	list := []TransitionData{}
	for _, t := range c.Transitions {
		list = append(list, transition(t))
	}
	e.Data = map[string]any{"issue": IssueData(c.Issue.Issue), "transitions": list}
	e.Meta["profile"] = profile
	return e
}
func TransitionsText(c app.Catalog) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  [%s]\n", c.Issue.Issue.Ref.Key, domain.CleanText(c.Issue.Issue.Status.Name, false))
	w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTRANSITION\tDESTINATION\tREQUIRED FIELDS")
	for _, t := range c.Transitions {
		required := []string{}
		for _, f := range t.Fields {
			if f.Required && !f.HasDefault {
				required = append(required, f.ID)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, domain.CleanText(t.Name, false), domain.CleanText(t.To.Name, false), strings.Join(required, ", "))
	}
	w.Flush()
	if len(c.Transitions) == 0 {
		b.WriteString("No transitions are currently available.\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
func PreparationEnvelope(p app.Preparation, err error) Envelope {
	e := Success(nil)
	if err != nil {
		e = Failure(err)
	}
	var selected any
	if p.Selected != nil {
		selected = transition(*p.Selected)
	}
	candidates := []TransitionData{}
	for _, t := range p.Candidates {
		candidates = append(candidates, transition(t))
	}
	missing := []string{}
	for _, f := range p.Missing {
		missing = append(missing, f.ID)
	}
	e.Data = map[string]any{"stage": "prepared", "issue": IssueData(p.Catalog.Issue.Issue), "intent": p.Intent, "transition": selected, "fields": p.Fields, "missing_fields": missing, "candidates": candidates, "noop": p.Noop}
	e.Meta["profile"] = p.Profile
	e.Meta["site_url"] = p.Site
	return e
}
func Preview(p app.Preparation) string {
	var b strings.Builder
	i := p.Catalog.Issue.Issue
	fmt.Fprintf(&b, "Site: %s | Profile: %s\n%s | %s\n", p.Site, p.Profile, i.Ref.Key, domain.CleanText(i.Summary, false))
	if p.Selected != nil {
		fmt.Fprintf(&b, "Transition: %s (ID %s)\nState: %s -> %s\n", domain.CleanText(p.Selected.Name, false), p.Selected.ID, domain.CleanText(i.Status.Name, false), domain.CleanText(p.Selected.To.Name, false))
	}
	if i.Resolution != nil {
		fmt.Fprintf(&b, "Current resolution: %s\n", domain.CleanText(i.Resolution.Name, false))
	} else {
		b.WriteString("Current resolution: none\n")
	}
	if len(p.Fields) > 0 {
		fields, _ := json.MarshalIndent(p.Fields, "", "  ")
		fmt.Fprintf(&b, "Requested fields:\n%s\n", fields)
	}
	if p.Noop {
		b.WriteString("No transition is needed; no write was sent.\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
func WorkflowEnvelope(r app.WorkflowResult, err error) Envelope {
	e := PreparationEnvelope(r.Plan, err)
	data := e.Data.(map[string]any)
	data["stage"] = "completed"
	data["result"] = r.Apply.State
	data["transition_id"] = r.Apply.TransitionID
	var before, observed any
	if r.Apply.Before != nil {
		before = status(*r.Apply.Before)
	}
	if r.Apply.Observed != nil {
		observed = status(*r.Apply.Observed)
	}
	data["before"] = before
	data["observed"] = observed
	e.Warnings = r.Warnings
	return e
}
func WorkflowText(r app.WorkflowResult) string {
	text := fmt.Sprintf("%s: %s", r.Apply.Issue.Key, r.Apply.State)
	if r.Apply.Observed != nil {
		text += " | observed state: " + domain.CleanText(r.Apply.Observed.Name, false)
	}
	if r.Plan.Catalog.Issue.Issue.Resolution != nil && r.Apply.State == domain.ApplyNoop {
		text += " | resolution: " + domain.CleanText(r.Plan.Catalog.Issue.Issue.Resolution.Name, false)
	}
	for _, warning := range r.Warnings {
		text += "\nWarning: " + warning
	}
	return text
}
