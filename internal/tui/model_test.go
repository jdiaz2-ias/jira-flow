package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type fakeReader struct {
	ctx    context.Context
	calls  int
	cursor string
}

func (f *fakeReader) Search(ctx context.Context, o app.SearchOptions) (app.SearchResult, error) {
	f.ctx = ctx
	f.calls++
	f.cursor = o.PageToken
	key := "APP-1"
	next := "next"
	if o.PageToken != "" {
		key = "APP-2"
		next = ""
	}
	return app.SearchResult{Issues: []domain.Issue{{Ref: domain.IssueRef{ID: "1", Key: key}, Summary: "Example"}}, Meta: app.ReadMeta{NextPageToken: next}}, nil
}
func (f *fakeReader) Show(ctx context.Context, key string, _ domain.DetailOptions, _, _ bool) (app.DetailResult, error) {
	f.ctx = ctx
	return app.DetailResult{Detail: domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "1", Key: key}}}}, nil
}
func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}
func TestNavigationPaginationAndFilter(t *testing.T) {
	f := &fakeReader{}
	m := New(context.Background(), f, Options{})
	m.Update(m.Init()())
	_, cmd := m.Update(key("n"))
	m.Update(cmd())
	if len(m.issues) != 2 || f.cursor != "next" {
		t.Fatal("pagination", m.issues, f.cursor)
	}
	m.Update(key("/"))
	m.Update(key("2"))
	m.Update(key("q"))
	if !m.filtering || m.filter != "2q" {
		t.Fatal("shortcut ran in text field")
	}
	m.Update(key("esc"))
	m.Update(key("j"))
	_, cmd = m.Update(key("enter"))
	m.Update(cmd())
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if !strings.Contains(m.View().Content, "APP-2") {
		t.Fatal("detail not rendered")
	}
	if !m.detailMode || m.detail.Detail.Issue.Ref.Key != "APP-2" {
		t.Fatal("wrong detail")
	}
	m.Update(key("esc"))
	if m.detailMode || m.selected != 1 {
		t.Fatal("back lost selection")
	}
}
func TestCancellationDiscardsOldResponses(t *testing.T) {
	f := &fakeReader{}
	m := New(context.Background(), f, Options{})
	old := m.Init()
	ctx := m.ctx
	m.Update(key("esc"))
	msg := old()
	if f.ctx.Err() != context.Canceled {
		t.Fatal("request was not canceled")
	}
	m.Update(msg)
	if len(m.issues) != 0 || m.busy || ctx.Err() != nil {
		t.Fatal("obsolete response accepted")
	}
}
func TestRefreshGenerationAndView(t *testing.T) {
	f := &fakeReader{}
	m := New(context.Background(), f, Options{})
	old := m.Init()
	_, newer := m.Update(key("r"))
	m.Update(newer())
	m.Update(old())
	if len(m.issues) != 1 || m.busy {
		t.Fatal("stale refresh overwrote model")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m.issues[0].Summary = "unsafe\x1b[31m\ntext"
	view := m.View()
	if !view.AltScreen || strings.Contains(view.Content, "\x1b") {
		t.Fatal("unsafe terminal content")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if !strings.Contains(m.View().Content, "Amplia") {
		t.Fatal("missing size guidance")
	}
}

func TestReadErrorsAndPartialMetadataStayVisible(t *testing.T) {
	m := New(context.Background(), &fakeReader{}, Options{})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.Update(resultMsg{generation: 0, search: app.SearchResult{Meta: app.ReadMeta{Stale: true}, Warnings: []string{"Partial page"}}, err: &domain.Error{Kind: domain.Partial, Message: "Read incomplete"}})
	view := m.View().Content
	if !strings.Contains(view, "Read incomplete") || !strings.Contains(view, "Partial page") || strings.Contains(view, "Sin resultados") {
		t.Fatal(view)
	}
}
