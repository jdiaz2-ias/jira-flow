// Package tui adapts keyboard interaction to application use cases.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type Reader interface {
	Search(context.Context, app.SearchOptions) (app.SearchResult, error)
	Show(context.Context, string, domain.DetailOptions, bool, bool) (app.DetailResult, error)
}
type Workflow interface {
	Prepare(context.Context, app.PrepareOptions) (app.Preparation, error)
	Apply(context.Context, app.Preparation) (app.WorkflowResult, error)
}
type Options struct {
	Search               app.SearchOptions
	Timeout              time.Duration
	Profile, Site, Theme string
	ASCII, NoColor       bool
	Workflow             Workflow
	Open                 func(context.Context, string) error
	Link                 func(string) (string, error)
}
type resultMsg struct {
	generation           uint64
	search               app.SearchResult
	detail               app.DetailResult
	isDetail, appendPage bool
	selectedKey          string
	err                  error
}
type Model struct {
	link                        string
	linkOffset                  int
	ctx                         context.Context
	reader                      Reader
	options                     Options
	cancel                      context.CancelFunc
	generation                  uint64
	busy                        bool
	issues                      []domain.Issue
	meta                        app.ReadMeta
	warnings                    []string
	selected, offset            int
	width, height               int
	filter                      string
	filtering, detailMode, help bool
	detail                      app.DetailResult
	detailKey                   string
	err                         error
	notice                      string
	remote, remoteBefore        string
	searching                   bool
	scopeLabel                  string
	dialog                      *actionDialog
	writing                     bool
	discard                     bool
	exitAfterDiscard            bool
	dark                        bool
}

func New(ctx context.Context, reader Reader, options Options) *Model {
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	return &Model{ctx: ctx, reader: reader, options: options, dark: true, scopeLabel: "Mis pendientes"}
}
func (m *Model) Init() tea.Cmd {
	load := m.load(false, false)
	if m.options.Theme == "auto" && !m.options.NoColor {
		return tea.Batch(load, tea.RequestBackgroundColor)
	}
	return load
}
func (m *Model) stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.generation++
	m.busy = false
}
func (m *Model) request() (context.Context, context.CancelFunc, uint64) {
	m.stop()
	ctx, cancel := context.WithTimeout(m.ctx, m.options.Timeout)
	m.cancel = cancel
	m.busy = true
	m.err = nil
	return ctx, cancel, m.generation
}
func (m *Model) selectedIssue() (domain.Issue, bool) {
	if m.detailMode && m.detailKey != "" {
		if m.detail.Detail.Issue.Ref.Key == m.detailKey {
			return m.detail.Detail.Issue, true
		}
		return domain.Issue{Ref: domain.IssueRef{Key: m.detailKey}}, true
	}
	items := m.visible()
	if m.selected < 0 || m.selected >= len(items) {
		return domain.Issue{}, false
	}
	return items[m.selected], true
}
func (m *Model) load(next, detail bool) tea.Cmd {
	issue, ok := m.selectedIssue()
	if detail && !ok {
		return nil
	}
	ctx, cancel, generation := m.request()
	opts := m.options.Search
	if next {
		opts.PageToken = m.meta.NextPageToken
		opts.Limit = min(opts.Limit, 5000-len(m.issues))
	}
	reader := m.reader
	return func() tea.Msg {
		defer cancel()
		msg := resultMsg{generation: generation, isDetail: detail, appendPage: next, selectedKey: issue.Ref.Key}
		if detail {
			msg.detail, msg.err = reader.Show(ctx, issue.Ref.Key, domain.DetailOptions{IncludeDescription: true, IncludeSubtasks: true, IncludeLinks: true}, opts.Refresh, opts.Offline)
		} else {
			msg.search, msg.err = reader.Search(ctx, opts)
		}
		return msg
	}
}
func (m *Model) visible() []domain.Issue {
	var items []domain.Issue
	for _, issue := range m.issues {
		if strings.Contains(strings.ToLower(issue.Ref.Key+" "+issue.Summary), strings.ToLower(m.filter)) {
			items = append(items, issue)
		}
	}
	return items
}
func (m *Model) enterDetail() tea.Cmd {
	issue, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	m.detailKey = issue.Ref.Key
	m.detail = app.DetailResult{}
	m.detailMode = true
	m.offset = 0
	return m.load(false, true)
}
func (m *Model) quit() tea.Cmd {
	if m.dialog != nil && m.dialog.dirty() {
		m.discard = true
		m.exitAfterDiscard = true
		return nil
	}
	m.stop()
	return tea.Quit
}
func editText(text string, key tea.KeyPressMsg, limit int) string {
	switch key.String() {
	case "backspace":
		r := []rune(text)
		if len(r) > 0 {
			return string(r[:len(r)-1])
		}
	case "ctrl+u":
		return ""
	default:
		if key.Text != "" && len(text)+len(key.Text) <= limit {
			return text + domain.CleanText(key.Text, false)
		}
	}
	return text
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.filtering || m.searching || (m.dialog != nil && m.dialog.stage == "field") {
			return m.Update(tea.KeyPressMsg{Text: string(msg.Content)})
		}
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case refreshedMsg:
		if msg.generation == m.generation {
			m.busy = false
			m.err = msg.err
			m.issues = msg.search.Issues
			m.meta = msg.search.Meta
			m.warnings = msg.search.Warnings
			if len(m.issues) >= 5000 && m.meta.NextPageToken != "" {
				m.warnings = append(m.warnings, "Limite de 5000 issues cargados; acota la busqueda JQL.")
			}
			m.detail = msg.detail
			m.selected = 0
			for i, issue := range m.visible() {
				if issue.Ref.Key == m.detailKey {
					m.selected = i
					break
				}
			}
		}
	case actionMsg:
		return m, m.receiveAction(msg)
	case openedMsg:
		if msg.generation == m.generation {
			m.busy = false
			m.err = msg.err
			if msg.err == nil {
				m.notice = "Navegador iniciado: " + msg.link
			}
		}
	case resultMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.busy = false
		m.err = msg.err
		if msg.isDetail {
			m.detail = msg.detail
		} else {
			if msg.appendPage {
				m.issues = append(m.issues, msg.search.Issues...)
			} else {
				m.issues = msg.search.Issues
				m.selected = 0
			}
			m.meta = msg.search.Meta
			m.warnings = msg.search.Warnings
			if len(m.issues) >= 5000 && m.meta.NextPageToken != "" {
				m.warnings = append(m.warnings, "Limite de 5000 issues cargados; acota la busqueda JQL.")
			}
			for i, issue := range m.visible() {
				if issue.Ref.Key == msg.selectedKey {
					m.selected = i
					break
				}
			}
		}
	case tea.KeyPressMsg:
		key := msg.String()
		// Once Apply starts its outcome must remain visible. Never discard its message,
		// start a second write, or refresh stale data over it while the request is pending.
		if m.writing {
			if key == "ctrl+c" && m.cancel != nil {
				m.cancel()
				m.notice = "Cancelando; esperando resultado. No reintentar a ciegas."
			}
			return m, nil
		}
		if m.width > 0 && m.height > 0 && (m.width < 60 || m.height < 15) && key != "q" && key != "ctrl+c" && key != "esc" {
			return m, nil
		}
		if m.discard {
			switch key {
			case "d":
				m.dialog = nil
				m.discard = false
				m.stop()
				if m.exitAfterDiscard {
					return m, tea.Quit
				}
			case "esc":
				m.discard = false
				m.exitAfterDiscard = false
			}
			return m, nil
		}
		if key == "ctrl+c" {
			return m, m.quit()
		}
		if m.dialog != nil {
			return m, m.dialogKey(msg)
		}
		if m.link != "" {
			switch key {
			case "esc", "y":
				m.link = ""
				m.linkOffset = 0
			case "q":
				return m, m.quit()
			case "o":
				return m, m.openIssue()
			case "j", "down":
				m.linkOffset = min(m.linkOffset+1, max(0, len(wrapped(m.link, m.width))-1))
			case "k", "up":
				m.linkOffset = max(0, m.linkOffset-1)
			}
			return m, nil
		}
		if m.searching {
			switch key {
			case "esc":
				m.searching = false
				m.remote = m.remoteBefore
			case "enter":
				jql, err := (domain.QueryOptions{Mode: "search", JQL: m.remote}).Build()
				if err != nil {
					m.err = err
					return m, nil
				}
				m.searching = false
				m.options.Search.Query.JQL = jql
				m.options.Search.PageToken = ""
				m.filter = ""
				m.selected = 0
				m.detailMode = false
				m.detailKey = ""
				m.scopeLabel = "Busqueda JQL"
				m.issues = nil
				m.meta = app.ReadMeta{}
				return m, m.load(false, false)
			default:
				m.remote = editText(m.remote, msg, 8192)
			}
			return m, nil
		}
		if m.filtering {
			switch key {
			case "enter":
				m.filtering = false
			case "esc":
				m.filtering = false
				m.filter = ""
			default:
				m.filter = editText(m.filter, msg, 256)
			}
			m.selected = 0
			return m, nil
		}
		switch key {
		case "q":
			return m, m.quit()
		case "?":
			m.help = !m.help
		case "esc":
			m.stop()
			m.detailMode = false
			m.detailKey = ""
			m.offset = 0
			m.err = nil
			m.help = false
		case "r":
			m.options.Search.Refresh = !m.options.Search.Offline
			return m, m.load(false, m.detailMode)
		case "n":
			if !m.busy && !m.detailMode && m.meta.NextPageToken != "" && len(m.issues) < 5000 {
				return m, m.load(true, false)
			}
		case "/":
			m.stop()
			m.detail = app.DetailResult{}
			m.detailMode = false
			m.detailKey = ""
			m.filtering = true
			m.selected = 0
		case "ctrl+f":
			m.stop()
			m.searching = true
			m.remoteBefore = m.remote
		case "enter":
			if !m.busy && !m.detailMode {
				return m, m.enterDetail()
			}
		case "tab", "shift+tab":
			if m.detailMode {
				m.stop()
				m.detailMode = false
				m.detailKey = ""
			} else if !m.busy {
				return m, m.enterDetail()
			}
		case "up", "k":
			if m.detailMode {
				if m.offset > 0 {
					m.offset--
				}
			} else if m.selected > 0 {
				m.selected--
				m.detail = app.DetailResult{}
			}
		case "down", "j":
			if m.detailMode {
				if m.offset < len(m.detailLines(max(1, m.detailWidth())))-1 {
					m.offset++
				}
			} else if m.selected+1 < len(m.visible()) {
				m.selected++
				m.detail = app.DetailResult{}
			}
		case "s":
			return m, m.startAction(domain.IntentStart)
		case "d":
			return m, m.startAction(domain.IntentDone)
		case "x":
			return m, m.startAction(domain.IntentClose)
		case "t":
			return m, m.startAction("")
		case "o":
			return m, m.openIssue()
		case "y":
			if issue, ok := m.selectedIssue(); ok && m.options.Link != nil {
				link, err := m.options.Link(issue.Ref.Key)
				m.err = err
				m.link = link
				m.linkOffset = 0
			}
		}
	}
	return m, nil
}
func freshness(meta app.ReadMeta) string {
	if meta.FetchedAt.IsZero() {
		return "Sin datos"
	}
	return fmt.Sprintf("%s | %s | obsoleto: %t | completo: %t", meta.Source, meta.FetchedAt.Format(time.RFC3339), meta.Stale, meta.Complete)
}

// PendingWrite is checked after the event loop exits so signal cancellation
// cannot be reported as an ordinary read cancellation when Jira may have changed.
func (m *Model) PendingWrite() bool { return m.writing }

// UncertainOutcome also covers a result received just before a signal ends the
// event loop. Acknowledging that result clears the dialog, not its visible notice.
func (m *Model) UncertainOutcome() bool {
	return m.dialog != nil && m.dialog.stage == "result" && (m.dialog.result.Apply.State == domain.ApplyUnknown || m.dialog.result.Apply.State == domain.ApplyAcceptedUnverified)
}
