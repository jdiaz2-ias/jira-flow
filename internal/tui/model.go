// Package tui adapts keyboard interaction to the existing reading use cases.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
)

type Reader interface {
	Search(context.Context, app.SearchOptions) (app.SearchResult, error)
	Show(context.Context, string, domain.DetailOptions, bool, bool) (app.DetailResult, error)
}

type Options struct {
	Search  app.SearchOptions
	Timeout time.Duration
	Profile string
}

type resultMsg struct {
	generation           uint64
	search               app.SearchResult
	detail               app.DetailResult
	isDetail, appendPage bool
	err                  error
}

type Model struct {
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
	err                         error
}

func New(ctx context.Context, reader Reader, options Options) *Model {
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	return &Model{ctx: ctx, reader: reader, options: options}
}

func (m *Model) Init() tea.Cmd { return m.load(false, false) }

func (m *Model) stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.generation++
	m.busy = false
}

func (m *Model) load(next, detail bool) tea.Cmd {
	m.stop()
	ctx, cancel := context.WithTimeout(m.ctx, m.options.Timeout)
	m.cancel = cancel
	m.busy = true
	m.err = nil
	generation := m.generation
	opts := m.options.Search
	if next {
		opts.PageToken = m.meta.NextPageToken
	}
	key := ""
	if detail {
		visible := m.visible()
		if len(visible) == 0 {
			m.stop()
			return nil
		}
		key = visible[m.selected].Ref.Key
	}
	return func() tea.Msg {
		defer cancel()
		msg := resultMsg{generation: generation, isDetail: detail, appendPage: next}
		if detail {
			msg.detail, msg.err = m.reader.Show(ctx, key, domain.DetailOptions{IncludeDescription: true, IncludeSubtasks: true, IncludeLinks: true}, opts.Refresh, opts.Offline)
		} else {
			msg.search, msg.err = m.reader.Search(ctx, opts)
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
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
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			m.stop()
			return m, tea.Quit
		}
		if m.filtering {
			switch key {
			case "enter":
				m.filtering = false
			case "esc":
				m.filtering = false
				m.filter = ""
			case "backspace":
				r := []rune(m.filter)
				if len(r) > 0 {
					m.filter = string(r[:len(r)-1])
				}
			default:
				if msg.Text != "" && len(m.filter)+len(msg.Text) <= 256 {
					m.filter += msg.Text
				}
			}
			m.selected = 0
			return m, nil
		}
		switch key {
		case "q":
			m.stop()
			return m, tea.Quit
		case "?":
			m.help = !m.help
		case "esc":
			m.stop()
			m.detailMode = false
			m.offset = 0
			m.err = nil
		case "r":
			m.options.Search.Refresh = !m.options.Search.Offline
			return m, m.load(false, m.detailMode)
		case "n":
			if !m.busy && !m.detailMode && m.meta.NextPageToken != "" && len(m.issues) < 5000 {
				return m, m.load(true, false)
			}
		case "/":
			if !m.detailMode {
				m.filtering = true
				m.selected = 0
			}
		case "enter":
			if !m.busy && !m.detailMode && len(m.visible()) > 0 {
				m.detailMode = true
				m.offset = 0
				return m, m.load(false, true)
			}
		case "up", "k":
			if m.detailMode {
				if m.offset > 0 {
					m.offset--
				}
			} else if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.detailMode {
				if m.offset < len(strings.Split(output.DetailText(m.detail), "\n"))-1 {
					m.offset++
				}
			} else if m.selected+1 < len(m.visible()) {
				m.selected++
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

func (m *Model) View() tea.View {
	var lines []string
	if m.width < 60 || m.height < 15 {
		lines = []string{"Amplia la terminal a 60x15 o usa jflow mine / show. q: salir"}
	} else {
		lines = append(lines, "Jira Flow | "+m.options.Profile+" | Mis pendientes")
		meta := m.meta
		if m.detailMode {
			meta = m.detail.Meta
		}
		lines = append(lines, freshness(meta))
		status := ""
		if m.busy {
			status = "Cargando... Esc: cancelar"
		}
		if m.err != nil {
			status = "Error: " + m.err.Error() + " | r: reintentar"
		}
		lines = append(lines, status)
		if m.help {
			lines = append(lines, "j/k o flechas: mover / desplazar detalle", "Enter: detalle | Esc: volver / cancelar", "/: filtro local | n: siguiente pagina | r: recargar", "q / Ctrl+C: salir | ?: ayuda", "Para transiciones usa jflow start/done/close.")
		} else if m.detailMode {
			body := strings.Split(output.DetailText(m.detail), "\n")
			start := min(m.offset, max(0, len(body)-1))
			lines = append(lines, body[start:min(len(body), start+m.height-5)]...)
		} else {
			lines = append(lines, fmt.Sprintf("Filtro local: %s | %d cargados | mas paginas: %t", m.filter, len(m.issues), m.meta.NextPageToken != ""))
			for _, warning := range m.warnings {
				lines = append(lines, "Aviso: "+warning)
			}
			items := m.visible()
			available := max(1, m.height-len(lines)-2)
			start := max(0, m.selected-available+1)
			for i := start; i < min(len(items), start+available); i++ {
				mark := "  "
				if i == m.selected {
					mark = "> "
				}
				issue := items[i]
				lines = append(lines, fmt.Sprintf("%s%s [%s] %s", mark, issue.Ref.Key, issue.Status.Name, issue.Summary))
			}
			if len(items) == 0 && !m.busy && m.err == nil {
				lines = append(lines, "Sin resultados para esta vista o filtro local.")
			}
		}
		lines = append(lines, "j/k mover | Enter detalle | / filtrar | n pagina | r recargar | ? ayuda | q salir")
	}
	// Remote content cannot inject terminal controls; crop by display cells, not bytes.
	for i, line := range lines {
		lines[i] = ansi.Truncate(domain.CleanText(line, false), max(1, m.width), "")
	}
	if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}
