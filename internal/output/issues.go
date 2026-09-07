package output

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type Named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Person struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}
type IssueStatus struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Category domain.StatusCategory `json:"category"`
}
type Issue struct {
	ID         string      `json:"id"`
	Key        string      `json:"key"`
	Summary    string      `json:"summary"`
	Status     IssueStatus `json:"status"`
	Resolution *Named      `json:"resolution"`
	Assignee   *Person     `json:"assignee"`
	Priority   *Named      `json:"priority"`
	IssueType  *Named      `json:"issue_type"`
	Project    *Named      `json:"project"`
	Updated    *string     `json:"updated_at"`
	DueDate    *string     `json:"due_date"`
	URL        string      `json:"url"`
}

func named(n *domain.NamedID) *Named {
	if n == nil {
		return nil
	}
	return &Named{n.ID, n.Name}
}
func person(n *domain.User) *Person {
	if n == nil {
		return nil
	}
	return &Person{n.ID, n.DisplayName}
}
func timestamp(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339Nano)
	return &s
}
func IssueData(i domain.Issue) Issue {
	var date *string
	if i.DueDate != nil {
		s := fmt.Sprintf("%04d-%02d-%02d", i.DueDate.Year, i.DueDate.Month, i.DueDate.Day)
		date = &s
	}
	return Issue{i.Ref.ID, i.Ref.Key, i.Summary, IssueStatus{i.Status.ID, i.Status.Name, i.Status.Category}, named(i.Resolution), person(i.Assignee), named(i.Priority), named(i.IssueType), named(i.Project), timestamp(i.UpdatedAt), date, i.URL}
}
func metadata(m app.ReadMeta) map[string]any {
	return map[string]any{"profile": m.Profile, "fetched_at": m.FetchedAt.Format(time.RFC3339Nano), "source": m.Source, "stale": m.Stale, "complete": m.Complete}
}
func SearchEnvelope(r app.SearchResult, err error) Envelope {
	e := Success(nil)
	if err != nil {
		e = Failure(err)
	}
	issues := []Issue{}
	for _, i := range r.Issues {
		issues = append(issues, IssueData(i))
	}
	e.Data = map[string]any{"issues": issues}
	e.Meta = metadata(r.Meta)
	e.Meta["returned"] = len(issues)
	if r.Meta.NextPageToken != "" {
		e.Meta["next_page_token"] = r.Meta.NextPageToken
	} else {
		e.Meta["next_page_token"] = nil
	}
	e.Warnings = r.Warnings
	return e
}

type Block struct {
	Kind     string  `json:"kind"`
	Text     string  `json:"text,omitempty"`
	URL      string  `json:"url,omitempty"`
	Children []Block `json:"children,omitempty"`
}

func blocks(in []domain.Block) []Block {
	out := []Block{}
	for _, b := range in {
		out = append(out, Block{b.Kind, b.Text, b.URL, blocks(b.Children)})
	}
	return out
}
func pageInfo(p domain.PageInfo) map[string]any {
	return map[string]any{"start_at": p.StartAt, "next_start": p.NextStart, "total": p.Total, "complete": p.Complete}
}
func DetailEnvelope(r app.DetailResult, err error) Envelope {
	e := Success(nil)
	if err != nil {
		e = Failure(err)
	}
	e.Meta = metadata(r.Meta)
	d := r.Detail
	e.Warnings = d.Warnings
	if e.Warnings == nil {
		e.Warnings = []string{}
	}
	if d.Issue.Ref.ID == "" {
		return e
	}
	subtasks := []Issue{}
	for _, i := range d.Subtasks {
		subtasks = append(subtasks, IssueData(i))
	}
	links := []map[string]string{}
	for _, link := range d.Links {
		links = append(links, map[string]string{"type": link.Type, "id": link.Target.ID, "key": link.Target.Key})
	}
	data := map[string]any{"issue": IssueData(d.Issue), "description": blocks(d.Description), "subtasks": subtasks, "subtasks_complete": d.SubtasksComplete, "links": links, "comments": nil, "history": nil}
	if d.Comments != nil {
		items := []map[string]any{}
		for _, item := range d.Comments.Items {
			items = append(items, map[string]any{"id": item.ID, "author": person(item.Author), "body": blocks(item.Body), "created_at": timestamp(item.CreatedAt)})
		}
		section := pageInfo(d.Comments.PageInfo)
		section["items"] = items
		data["comments"] = section
	}
	if d.History != nil {
		items := []map[string]any{}
		for _, item := range d.History.Items {
			changes := []map[string]string{}
			for _, change := range item.Changes {
				changes = append(changes, map[string]string{"field": change.Field, "from": change.From, "to": change.To})
			}
			items = append(items, map[string]any{"id": item.ID, "author": person(item.Author), "created_at": timestamp(item.CreatedAt), "changes": changes})
		}
		section := pageInfo(d.History.PageInfo)
		section["items"] = items
		data["history"] = section
	}
	e.Data = data
	return e
}
func Rows(r app.SearchResult, table bool) string {
	var b strings.Builder
	clean := func(s string) string { return domain.CleanText(s, false) }
	if table {
		w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "CLAVE\tESTADO\tPRIORIDAD\tASIGNADO\tRESUMEN")
		for _, i := range r.Issues {
			priority := "-"
			if i.Priority != nil {
				priority = i.Priority.Name
			}
			assignee := "Sin asignar"
			if i.Assignee != nil {
				assignee = i.Assignee.DisplayName
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", i.Ref.Key, clean(i.Status.Name), clean(priority), clean(assignee), clean(i.Summary))
		}
		w.Flush()
	} else {
		for _, i := range r.Issues {
			fmt.Fprintf(&b, "%s  [%s]  %s\n", i.Ref.Key, clean(i.Status.Name), clean(i.Summary))
		}
	}
	if len(r.Issues) == 0 {
		b.WriteString("Sin resultados.\n")
	}
	if !r.Meta.Complete {
		fmt.Fprintf(&b, "Resultado incompleto: %d issues cargados; total desconocido.\n", len(r.Issues))
		if r.Meta.NextPageToken != "" {
			fmt.Fprintf(&b, "Continúa con --page-token '%s'\n", r.Meta.NextPageToken)
		}
	}
	if r.Meta.Stale {
		b.WriteString("Datos de memoria vencidos.\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
func BlocksText(in []domain.Block) string {
	var b strings.Builder
	var write func(domain.Block, int)
	write = func(n domain.Block, depth int) {
		if depth > 64 {
			return
		}
		if n.Kind == "listItem" {
			b.WriteString("- ")
		}
		b.WriteString(domain.CleanText(n.Text, true))
		for _, child := range n.Children {
			write(child, depth+1)
		}
		if n.URL != "" && n.URL != n.Text {
			b.WriteString(" (" + domain.CleanText(n.URL, false) + ")")
		}
		switch n.Kind {
		case "paragraph", "heading", "listItem", "codeBlock", "blockquote", "tableRow":
			b.WriteByte('\n')
		case "tableCell", "tableHeader":
			b.WriteByte('\t')
		}
	}
	for _, n := range in {
		write(n, 0)
	}
	return strings.TrimSpace(b.String())
}
func DetailText(r app.DetailResult) string {
	d := r.Detail
	if d.Issue.Ref.ID == "" {
		return ""
	}
	var b strings.Builder
	clean := func(s string) string { return domain.CleanText(s, false) }
	fmt.Fprintf(&b, "%s  %s\nEstado: %s (%s)\n", d.Issue.Ref.Key, clean(d.Issue.Summary), clean(d.Issue.Status.Name), d.Issue.Status.Category)
	resolution := "Sin resolución"
	if d.Issue.Resolution != nil {
		resolution = clean(d.Issue.Resolution.Name)
	}
	fmt.Fprintf(&b, "Resolución: %s\n%s\n", resolution, d.Issue.URL)
	if text := BlocksText(d.Description); text != "" {
		fmt.Fprintf(&b, "\nDescripción:\n%s\n", text)
	}
	if len(d.Subtasks) > 0 {
		b.WriteString("\nSubtareas visibles:\n")
		for _, i := range d.Subtasks {
			fmt.Fprintf(&b, "%s  [%s]  %s\n", i.Ref.Key, clean(i.Status.Name), clean(i.Summary))
		}
	}
	if len(d.Links) > 0 {
		b.WriteString("\nEnlaces:\n")
		for _, link := range d.Links {
			fmt.Fprintf(&b, "%s: %s\n", clean(link.Type), link.Target.Key)
		}
	}
	if d.Comments != nil {
		b.WriteString("\nComentarios:\n")
		for _, c := range d.Comments.Items {
			name := "Autor desconocido"
			if c.Author != nil {
				name = clean(c.Author.DisplayName)
			}
			fmt.Fprintf(&b, "[%s] %s\n%s\n", clean(c.ID), name, BlocksText(c.Body))
		}
		if !d.Comments.Complete {
			b.WriteString("Comentarios incompletos.")
			if d.Comments.NextStart != nil {
				fmt.Fprintf(&b, " Continúa con --comments-start %d", *d.Comments.NextStart)
			}
			b.WriteByte('\n')
		}
	}
	if d.History != nil {
		b.WriteString("\nHistorial:\n")
		for _, h := range d.History.Items {
			for _, change := range h.Changes {
				fmt.Fprintf(&b, "[%s] %s: %s -> %s\n", clean(h.ID), clean(change.Field), clean(change.From), clean(change.To))
			}
		}
		if !d.History.Complete {
			b.WriteString("Historial incompleto.")
			if d.History.NextStart != nil {
				fmt.Fprintf(&b, " Continúa con --history-start %d", *d.History.NextStart)
			}
			b.WriteByte('\n')
		}
	}
	for _, warning := range d.Warnings {
		fmt.Fprintf(&b, "Aviso: %s\n", warning)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
