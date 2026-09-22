package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
)

type actionDialog struct {
	options        app.PrepareOptions
	plan           app.Preparation
	stage          string
	choice, scroll int
	field          domain.FieldSpec
	input          string
	result         app.WorkflowResult
	resultErr      error
}

func (d *actionDialog) dirty() bool {
	return d.stage != "result" && (len(d.options.Fields) > 0 || d.input != "")
}

type actionMsg struct {
	generation uint64
	plan       app.Preparation
	result     app.WorkflowResult
	applied    bool
	err        error
}

func (m *Model) startAction(intent domain.Intent) tea.Cmd {
	if m.busy {
		return nil
	}
	if m.options.Search.Offline {
		m.err = &domain.Error{Kind: domain.InvalidInput, Message: "Las transiciones requieren conexion; sal del modo offline."}
		return nil
	}
	if m.options.Workflow == nil {
		m.err = &domain.Error{Kind: domain.Unsupported, Message: "Workflow no disponible."}
		return nil
	}
	issue, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	m.dialog = &actionDialog{options: app.PrepareOptions{Key: issue.Ref.Key, Intent: intent, Fields: map[string]domain.FieldValue{}}}
	return m.prepare()
}
func (m *Model) prepare() tea.Cmd {
	ctx, cancel, generation := m.request()
	d := m.dialog
	d.stage = "loading"
	d.scroll = 0
	opts := d.options
	// Own the map used by the command; text editing never races with HTTP work.
	opts.Fields = make(map[string]domain.FieldValue, len(d.options.Fields))
	for k, v := range d.options.Fields {
		opts.Fields[k] = v
	}
	workflow := m.options.Workflow
	return func() tea.Msg {
		defer cancel()
		p, err := workflow.Prepare(ctx, opts)
		return actionMsg{generation: generation, plan: p, err: err}
	}
}
func (m *Model) apply() tea.Cmd {
	ctx, cancel, generation := m.request()
	m.writing = true
	m.dialog.stage = "writing"
	plan := m.dialog.plan
	workflow := m.options.Workflow
	return func() tea.Msg {
		defer cancel()
		result, err := workflow.Apply(ctx, plan)
		return actionMsg{generation: generation, result: result, err: err, applied: true}
	}
}
func (m *Model) receiveAction(msg actionMsg) tea.Cmd {
	if msg.generation != m.generation || m.dialog == nil {
		return nil
	}
	m.busy = false
	m.err = msg.err
	d := m.dialog
	d.scroll = 0
	if msg.applied {
		m.writing = false
		d.stage = "result"
		d.result = msg.result
		d.resultErr = msg.err
		m.notice = output.WorkflowText(msg.result)
		if msg.err != nil {
			m.notice += "\n" + msg.err.Error()
		}
		// Display the exact outcome until acknowledged; refresh is a separate read.
		return nil
	}
	d.plan = msg.plan
	if msg.plan.Selected != nil {
		d.options.TransitionID = msg.plan.Selected.ID
	}
	var public *domain.Error
	switch {
	case msg.err == nil:
		d.stage = "review"
		d.choice = 0
	case errors.As(msg.err, &public) && public.Kind == domain.TransitionAmbiguous && len(msg.plan.Candidates) > 0:
		d.stage = "choose"
		d.choice = 0
		m.err = nil
	case public != nil && public.Kind == domain.Validation && len(msg.plan.Missing) > 0:
		m.editField(msg.plan.Missing[0])
		m.err = nil
	default:
		d.stage = "error"
	}
	return nil
}
func (m *Model) editField(field domain.FieldSpec) {
	d := m.dialog
	d.field = field
	d.input = ""
	d.stage = "field"
	d.scroll = 0
}
func (m *Model) dismissDialog() tea.Cmd {
	key := m.dialog.options.Key
	m.dialog = nil
	m.err = nil
	m.detailMode = true
	m.detailKey = key
	m.detail = app.DetailResult{}
	m.offset = 0
	m.options.Search.Refresh = true
	// Refresh list and detail after every result, including unknown outcomes. Never
	// infer success from a later read; the original outcome remains in notice.
	ctx, cancel, generation := m.request()
	reader := m.reader
	opts := m.options.Search
	opts.PageToken = ""
	opts.Refresh = true
	return func() tea.Msg {
		defer cancel()
		search, err := reader.Search(ctx, opts)
		detail, detailErr := reader.Show(ctx, key, domain.DetailOptions{IncludeDescription: true, IncludeSubtasks: true, IncludeLinks: true}, true, false)
		return refreshedMsg{generation: generation, search: search, detail: detail, err: errors.Join(err, detailErr)}
	}
}

type refreshedMsg struct {
	generation uint64
	search     app.SearchResult
	detail     app.DetailResult
	err        error
}

func (m *Model) dialogKey(msg tea.KeyPressMsg) tea.Cmd {
	d := m.dialog
	key := msg.String()
	if key == "esc" {
		if d.stage == "result" {
			return m.dismissDialog()
		}
		if d.dirty() {
			m.discard = true
			m.exitAfterDiscard = false
			return nil
		}
		m.stop()
		m.dialog = nil
		m.err = nil
		return nil
	}
	// Field content owns letter keys, including q, y, and mutation shortcuts.
	if d.stage == "field" {
		switch key {
		case "enter":
			value, err := domain.ParseFieldInput(d.field, d.input)
			if err != nil {
				m.err = err
				return nil
			}
			d.options.Fields[d.field.ID] = value
			d.input = ""
			return m.prepare()
		case "pgdown":
			d.scroll++
		case "pgup":
			if d.scroll > 0 {
				d.scroll--
			}
		default:
			d.input = editText(d.input, msg, 65536)
		}
		return nil
	}
	if key == "q" {
		return m.quit()
	}
	if m.busy {
		return nil
	}
	if key == "o" {
		return m.openIssue()
	}
	switch d.stage {
	case "choose", "fields":
		count := len(d.plan.Candidates)
		if d.stage == "fields" {
			count = len(d.plan.Selected.Fields)
		}
		switch key {
		case "up", "k":
			if d.choice > 0 {
				d.choice--
			}
		case "down", "j":
			if d.choice+1 < count {
				d.choice++
			}
		case "enter":
			if count == 0 {
				return nil
			}
			if d.stage == "choose" {
				d.options.TransitionID = d.plan.Candidates[d.choice].ID
				return m.prepare()
			}
			m.editField(d.plan.Selected.Fields[d.choice])
		}
	case "review":
		switch key {
		case "e":
			if d.plan.Selected != nil && len(d.plan.Selected.Fields) > 0 {
				d.stage = "fields"
				d.choice = 0
				d.scroll = 0
			}
		case "y":
			if !m.reviewVisible() {
				m.err = &domain.Error{Kind: domain.InvalidInput, Message: "Revisa hasta el final con j / PageDown antes de confirmar."}
				return nil
			}
			return m.apply()
		case "enter": // Enter is deliberately not an affirmative confirmation.
			if d.plan.Noop {
				m.notice = output.Preview(d.plan)
				return m.dismissDialog()
			}
		case "n":
			if d.dirty() {
				m.discard = true
				m.exitAfterDiscard = false
			} else {
				m.dialog = nil
				m.err = nil
			}
		}
	case "result":
		if key == "enter" {
			return m.dismissDialog()
		}
	case "error":
		if key == "r" {
			return m.prepare()
		}
	}
	if key == "down" || key == "j" || key == "pgdown" {
		if d.stage != "choose" && d.stage != "fields" {
			d.scroll = min(d.scroll+1, max(0, len(m.dialogLines())-1))
		}
	}
	if key == "up" || key == "k" || key == "pgup" {
		if d.scroll > 0 {
			d.scroll--
		}
	}
	return nil
}
func (m *Model) reviewVisible() bool {
	return m.dialog.scroll+max(1, m.height-7) >= len(m.dialogLines())
}
func (m *Model) dialogLines() []string {
	d := m.dialog
	if d == nil {
		return nil
	}
	var text string
	switch d.stage {
	case "loading":
		text = "Preparando accion; ninguna escritura enviada."
	case "writing":
		text = "Enviando una sola transicion y verificando...\nCtrl+C cancela la espera; el resultado puede ser incierto."
	case "choose":
		text = "Selecciona una transicion disponible (Enter):"
		for i, t := range d.plan.Candidates {
			mark := "  "
			if i == d.choice {
				mark = "> "
			}
			text += fmt.Sprintf("\n%s%s: %s -> %s", mark, t.ID, t.Name, t.To.Name)
		}
	case "fields":
		text = "Selecciona un campo para editar (Enter):"
		for i, f := range d.plan.Selected.Fields {
			mark := "  "
			if i == d.choice {
				mark = "> "
			}
			text += fmt.Sprintf("\n%s%s (%s), requerido: %t", mark, f.ID, f.Type, f.Required)
		}
	case "field":
		text = fmt.Sprintf("Campo: %s [%s, %s] requerido: %t\n", d.field.Name, d.field.ID, d.field.Type, d.field.Required)
		for _, v := range d.field.AllowedValues {
			text += v.ID + " = " + v.Name + "\n"
		}
		text += "Valor: " + d.input + "\nEnter: validar | Ctrl+U: limpiar | Esc: cancelar"
		if d.field.Type == "array" {
			text += "\nIntroduce un arreglo JSON de valores u objetos ID."
		}
	case "review":
		text = "Revisar antes de aplicar:\n" + output.Preview(d.plan) + "\n[y] Aplicar  [n/Esc] Cancelar  [e] Editar campos\nEnter no confirma. j/k: desplazar la revision."
	case "result":
		text = output.WorkflowText(d.result)
		if d.resultErr != nil {
			text += "\n" + d.resultErr.Error()
		}
		text += "\nEnter: actualizar vista | o: abrir Jira | Esc: volver"
	case "error":
		text = "No se envio ningun cambio.\n" + output.Preview(d.plan) + "\nr: preparar nuevamente | o: abrir Jira | Esc: volver"
	}
	return wrapped(text, max(1, m.width-2))
}

type openedMsg struct {
	generation uint64
	link       string
	err        error
}

// The browser runs while Bubble Tea has released the terminal; it is restored
// before the callback is displayed, including on launcher failure.
type browserCommand struct {
	ctx    context.Context
	open   func(context.Context, string) error
	link   string
	cancel context.CancelFunc
}

func (b browserCommand) Run() error        { defer b.cancel(); return b.open(b.ctx, b.link) }
func (browserCommand) SetStdin(io.Reader)  {}
func (browserCommand) SetStdout(io.Writer) {}
func (browserCommand) SetStderr(io.Writer) {}
func (m *Model) openIssue() tea.Cmd {
	if m.busy || m.options.Open == nil || m.options.Link == nil {
		return nil
	}
	key := ""
	if m.dialog != nil {
		key = m.dialog.options.Key
	} else if issue, ok := m.selectedIssue(); ok {
		key = issue.Ref.Key
	}
	if key == "" {
		return nil
	}
	link, err := m.options.Link(key)
	if err != nil {
		m.err = err
		return nil
	}
	ctx, cancel, generation := m.request()
	return tea.Exec(browserCommand{ctx: ctx, open: m.options.Open, link: link, cancel: cancel}, func(err error) tea.Msg { return openedMsg{generation: generation, link: link, err: err} })
}
func wrapped(text string, width int) []string {
	// Sanitize each logical line before wrapping; preserve intentional newlines.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = domain.CleanText(line, false)
	}
	return wrapLines(lines, width)
}
