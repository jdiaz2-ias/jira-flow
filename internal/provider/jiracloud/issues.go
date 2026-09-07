package jiracloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

// Session binds provider ports to one profile and credential for an invocation.
type Session struct {
	Client  *Client
	Profile config.Profile
	Secret  ports.Secret
}

var _ ports.IssueReader = (*Session)(nil)

func (s *Session) Myself(ctx context.Context) (domain.User, error) {
	return s.Client.Myself(ctx, s.Profile, s.Secret)
}

var ListFields = []string{"summary", "status", "assignee", "priority", "issuetype", "project", "updated", "duedate", "resolution"}

type wireNamed struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}
type wireUser struct {
	ID   string `json:"accountId"`
	Name string `json:"displayName"`
}
type wireIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Resolution  *wireNamed      `json:"resolution"`
		Assignee    *wireUser       `json:"assignee"`
		Project     *wireNamed      `json:"project"`
		IssueType   *wireNamed      `json:"issuetype"`
		Priority    *wireNamed      `json:"priority"`
		Updated     string          `json:"updated"`
		DueDate     string          `json:"duedate"`
		Description json.RawMessage `json:"description"`
		Subtasks    []wireIssue     `json:"subtasks"`
		Links       []struct {
			Type    struct{ Inward, Outward string }
			Inward  *wireIssue `json:"inwardIssue"`
			Outward *wireIssue `json:"outwardIssue"`
		} `json:"issuelinks"`
	} `json:"fields"`
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999-0700"} {
		if value, err := time.Parse(layout, s); err == nil {
			return value
		}
	}
	return time.Time{}
}
func named(n *wireNamed, clean func(string) string) *domain.NamedID {
	if n == nil {
		return nil
	}
	name := n.Name
	if name == "" {
		name = n.Key
	}
	return &domain.NamedID{ID: clean(n.ID), Name: clean(name)}
}
func user(n *wireUser, clean func(string) string) *domain.User {
	if n == nil {
		return nil
	}
	return &domain.User{ID: clean(n.ID), DisplayName: clean(n.Name)}
}
func (s *Session) issue(w wireIssue) (domain.Issue, error) {
	key, err := domain.IssueKey(w.Key)
	if err != nil || w.ID == "" {
		return domain.Issue{}, failure(domain.Unavailable, "Jira returned an issue without a valid identifier or key.")
	}
	clean := cleanRemote(s.Profile, s.Secret)
	category, ok := map[string]domain.StatusCategory{"new": domain.CategoryTodo, "indeterminate": domain.CategoryInProgress, "done": domain.CategoryDone}[w.Fields.Status.Category.Key]
	if !ok {
		category = domain.CategoryUnknown
	}
	link, _ := domain.BrowseURL(s.Profile.SiteURL, key)
	result := domain.Issue{Ref: domain.IssueRef{ID: clean(w.ID), Key: key}, Summary: clean(w.Fields.Summary), Status: domain.Status{ID: clean(w.Fields.Status.ID), Name: clean(w.Fields.Status.Name), Category: category}, Resolution: named(w.Fields.Resolution, clean), Assignee: user(w.Fields.Assignee, clean), UpdatedAt: parseTime(w.Fields.Updated), URL: link, Priority: named(w.Fields.Priority, clean), IssueType: named(w.Fields.IssueType, clean), Project: named(w.Fields.Project, clean)}
	if result.Project != nil {
		result.ProjectID = result.Project.ID
	}
	if result.IssueType != nil {
		result.IssueTypeID = result.IssueType.ID
	}
	if date, e := time.Parse("2006-01-02", w.Fields.DueDate); e == nil {
		result.DueDate = &domain.LocalDate{Year: date.Year(), Month: date.Month(), Day: date.Day()}
	}
	return result, nil
}
func (s *Session) Search(ctx context.Context, q domain.SearchRequest) (domain.IssuePage, error) {
	result := domain.IssuePage{Issues: []domain.Issue{}}
	if q.PageSize < 1 || q.PageSize > 100 || len(q.PageToken) > 32768 {
		return result, failure(domain.InvalidInput, "Invalid page size or cursor.")
	}
	fields := q.Fields
	if len(fields) == 0 {
		fields = ListFields
	}
	for _, field := range fields {
		allowed := false
		for _, known := range ListFields {
			if field == known {
				allowed = true
			}
		}
		if !allowed {
			return result, failure(domain.InvalidInput, "List field not allowed.")
		}
	}
	body := struct {
		JQL    string   `json:"jql"`
		Fields []string `json:"fields"`
		Max    int      `json:"maxResults"`
		Token  string   `json:"nextPageToken,omitempty"`
	}{q.JQL, fields, q.PageSize, q.PageToken}
	b, err := s.Client.read(ctx, s.Profile, s.Secret, http.MethodPost, "/rest/api/3/search/jql", body)
	if err != nil {
		return result, err
	}
	var page struct {
		Issues []wireIssue `json:"issues"`
		Next   string      `json:"nextPageToken"`
		Last   *bool       `json:"isLast"`
	}
	if json.Unmarshal(b, &page) != nil || page.Issues == nil || len(page.Issues) > 100 || len(page.Next) > 32768 {
		return result, failure(domain.Unavailable, "Invalid Jira search page.")
	}
	for _, w := range page.Issues {
		issue, err := s.issue(w)
		if err != nil {
			return result, err
		}
		result.Issues = append(result.Issues, issue)
	}
	result.Complete = page.Last != nil && *page.Last
	result.NextPageToken = page.Next
	if page.Last == nil && page.Next == "" {
		return result, failure(domain.Unavailable, "Jira did not indicate whether the page is complete.")
	}
	if result.Complete {
		result.NextPageToken = ""
	} else if result.NextPageToken == "" {
		return result, failure(domain.Unavailable, "Jira indicated more results but provided no cursor.")
	}
	return result, nil
}
func (s *Session) GetIssue(ctx context.Context, ref domain.IssueRef, options domain.DetailOptions) (domain.IssueDetail, error) {
	result := domain.IssueDetail{Description: []domain.Block{}, Subtasks: []domain.Issue{}, Links: []domain.IssueLink{}, Warnings: []string{}}
	key, err := domain.IssueKey(ref.Key)
	if err != nil {
		return result, err
	}
	fields := append([]string(nil), ListFields...)
	for _, id := range options.Fields {
		if !domain.ValidFieldID(id) || id == "comment" {
			return result, failure(domain.InvalidInput, "Invalid requested verification field.")
		}
		fields = append(fields, id)
	}
	if options.IncludeDescription {
		fields = append(fields, "description")
	}
	if options.IncludeSubtasks {
		fields = append(fields, "subtasks")
	}
	if options.IncludeLinks {
		fields = append(fields, "issuelinks")
	}
	b, err := s.Client.read(ctx, s.Profile, s.Secret, http.MethodGet, "/rest/api/3/issue/"+key+"?"+url.Values{"fields": {strings.Join(fields, ",")}}.Encode(), nil)
	if err != nil {
		return result, err
	}
	var w wireIssue
	if json.Unmarshal(b, &w) != nil {
		return result, failure(domain.Unavailable, "Invalid Jira detail.")
	}
	var values struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if json.Unmarshal(b, &values) != nil {
		return result, failure(domain.Unavailable, "Invalid issue fields.")
	}
	result.Values = map[string]domain.FieldValue{}
	for _, id := range options.Fields {
		if raw, ok := values.Fields[id]; ok {
			var value any
			if json.Unmarshal(raw, &value) != nil {
				return result, failure(domain.Unavailable, "Invalid verification field value.")
			}
			result.Values[id] = value
		}
	}
	result.Issue, err = s.issue(w)
	if err != nil {
		return result, err
	}
	clean := cleanRemote(s.Profile, s.Secret)
	if options.IncludeDescription {
		result.Description, result.Warnings = normalizeADF(w.Fields.Description, clean)
	}
	if options.IncludeSubtasks {
		result.SubtasksComplete = w.Fields.Subtasks != nil
		for _, sub := range w.Fields.Subtasks {
			issue, e := s.issue(sub)
			if e != nil {
				result.SubtasksComplete = false
				result.Warnings = append(result.Warnings, "Jira returned an incomplete subtask.")
				continue
			}
			result.Subtasks = append(result.Subtasks, issue)
		}
	}
	if options.IncludeLinks {
		for _, link := range w.Fields.Links {
			target := link.Outward
			label := link.Type.Outward
			if target == nil {
				target = link.Inward
				label = link.Type.Inward
			}
			if target == nil {
				continue
			}
			key, e := domain.IssueKey(target.Key)
			if e != nil {
				continue
			}
			result.Links = append(result.Links, domain.IssueLink{Type: clean(label), Target: domain.IssueRef{ID: clean(target.ID), Key: key}})
		}
	}
	if options.IncludeComments {
		page, e := s.comments(ctx, key, options)
		result.Comments = &page
		if e != nil {
			return result, partialDetail(e)
		}
	}
	if options.IncludeHistory {
		page, e := s.history(ctx, key, options)
		result.History = &page
		if e != nil {
			return result, partialDetail(e)
		}
	}
	return result, nil
}
func partialDetail(err error) error {
	var public *domain.Error
	if errors.As(err, &public) && public.Kind == domain.Canceled {
		return err
	}
	return &domain.Error{Kind: domain.Partial, Message: "The issue was loaded, but a requested section remained incomplete: " + err.Error(), Cause: err}
}
