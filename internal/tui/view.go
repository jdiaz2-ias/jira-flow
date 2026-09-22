package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
)

func wrapLines(lines []string, width int) []string {
	var out []string
	for _, line := range lines {
		out = append(out, strings.Split(ansi.Hardwrap(line, max(1, width), false), "\n")...)
	}
	return out
}
func (m *Model) detailWidth() int {
	if m.width >= 110 {
		return m.width - (m.width * 55 / 100) - 3
	}
	return m.width
}
func (m *Model) detailLines(width int) []string {
	if m.detail.Detail.Issue.Ref.ID != "" {
		metrics := domain.MeasureProgress(m.detail.Detail, time.Now(), time.Local)
		text := output.ProgressText(m.detail, metrics, time.Local.String()) + "\n\n" + output.DetailText(m.detail)
		if m.options.ASCII {
			text = strings.ReplaceAll(text, " · ", " | ")
		}
		return wrapped(text, width)
	}
	if m.detailMode && m.busy {
		return []string{"Cargando detalle..."}
	}
	issue, ok := m.selectedIssue()
	if !ok {
		return []string{"Selecciona un issue."}
	}
	return wrapped(fmt.Sprintf("%s\n%s\nEstado: %s (%s)\nEnter / Tab: cargar detalle", issue.Ref.Key, issue.Summary, issue.Status.Name, issue.Status.Category), width)
}
func (m *Model) listLines(width, height int) []string {
	items := m.visible()
	lines := []string{}
	start := max(0, m.selected-height+1)
	for i := start; i < min(len(items), start+height); i++ {
		mark := "  "
		if i == m.selected {
			mark = "> "
		}
		issue := items[i]
		value := fmt.Sprintf("%s%s [%s] %s", mark, issue.Ref.Key, issue.Status.Name, issue.Summary)
		if width < 80 {
			value = fmt.Sprintf("%s%s [%s] %s", mark, issue.Ref.Key, issue.Status.Category, issue.Summary)
		}
		line := ansi.Truncate(domain.CleanText(value, false), width, "")
		color := ""
		switch issue.Status.Category {
		case domain.CategoryInProgress:
			color = "accent"
		case domain.CategoryDone:
			color = "done"
		}
		lines = append(lines, m.paint(line, color))
	}
	if len(items) == 0 && !m.busy && m.err == nil {
		lines = append(lines, "Sin resultados para esta vista o filtro local.")
	}
	return lines
}
func (m *Model) paint(text, kind string) string {
	if m.options.NoColor || m.options.Theme == "mono" || m.options.Theme == "" || kind == "" {
		return text
	}
	dark := m.dark
	if m.options.Theme == "dark" {
		dark = true
	}
	if m.options.Theme == "light" {
		dark = false
	}
	colors := map[string]string{"accent": "#005F87", "done": "#006B3C", "error": "#B00020", "warning": "#805500"}
	if dark {
		colors = map[string]string{"accent": "#66CCFF", "done": "#73D69B", "error": "#FF8C8C", "warning": "#FFD166"}
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colors[kind])).Render(text)
}
func (m *Model) View() tea.View {
	if m.width < 60 || m.height < 15 {
		v := tea.NewView(ansi.Truncate("Amplia la terminal a 60x15 o usa jflow mine / show. q: salir", max(1, m.width), ""))
		v.AltScreen = true
		return v
	}
	clean := func(s string) string { return ansi.Truncate(domain.CleanText(s, false), m.width, "") }
	focus := "lista"
	if m.detailMode {
		focus = "detalle"
	}
	lines := []string{m.paint(clean("Jira Flow | "+m.options.Profile+" | "+m.options.Site+" | "+m.scopeLabel+" | foco: "+focus), "accent")}
	meta := m.meta
	if m.detailMode {
		meta = m.detail.Meta
	}
	lines = append(lines, clean(freshness(meta)))
	status := ""
	if m.busy {
		status = "Cargando... Esc: cancelar"
	}
	if m.writing {
		status = "Enviando/verificando; acciones bloqueadas hasta recibir resultado."
	}
	if m.err != nil {
		status = "Error: " + m.err.Error()
	}
	lines = append(lines, m.paint(clean(status), "error"))
	if m.discard {
		lines = append(lines, "Hay campos sin publicar. d: descartar | Esc: conservar y volver")
	} else if m.dialog != nil {
		body := m.dialogLines()
		room := max(1, m.height-len(lines)-3)
		if m.dialog.stage == "field" {
			room = max(1, room-2)
		}
		start := min(m.dialog.scroll, max(0, len(body)-1))
		if m.dialog.stage == "choose" || m.dialog.stage == "fields" {
			start = max(0, m.dialog.choice-room+2)
		}
		lines = append(lines, body[start:min(len(body), start+room)]...)
		if m.dialog.stage == "field" {
			value := wrapped("Valor: "+m.dialog.input, m.width)
			lines = append(lines, value[max(0, len(value)-2):]...)
		}
		lines = append(lines, fmt.Sprintf("Revision %d-%d de %d | j/k: desplazar | Esc: volver", start+1, min(len(body), start+room), len(body)))
	} else if m.link != "" {
		body := wrapped(m.link, m.width)
		start := min(m.linkOffset, len(body)-1)
		lines = append(lines, "Enlace seleccionable (sin copia automatica):")
		lines = append(lines, body[start:min(len(body), start+m.height-6)]...)
		lines = append(lines, "Esc: volver | o: abrir | j/k: desplazar")
	} else if m.searching {
		lines = append(lines, "Busqueda remota JQL (consulta Jira al pulsar Enter):")
		body := wrapped(m.remote, m.width)
		room := max(1, m.height-len(lines)-2)
		lines = append(lines, body[max(0, len(body)-room):]...)
		lines = append(lines, "Enter: buscar | Esc: cancelar | Ctrl+U: limpiar")
	} else if m.help {
		lines = append(lines, "j/k o flechas: seleccionar / desplazar | Enter: detalle", "Tab / Shift+Tab: foco lista/detalle | Esc: volver/cancelar", "/: filtro local | Ctrl+F: busqueda JQL remota", "s: iniciar | d: completar | x: cerrar | t: transiciones", "Las acciones muestran revision; solo y confirma.", "o: navegador | y: mostrar enlace | n: pagina | r: recargar", "q / Ctrl+C: salir (protege campos sin publicar)", "?: ayuda | --theme auto|dark|light|mono | --ascii")
	} else {
		lines = append(lines, clean(fmt.Sprintf("Filtro local: %s | %d cargados | mas paginas: %t", m.filter, len(m.issues), m.meta.NextPageToken != "")))
		// Keep warning and outcome summaries visible without starving the panels.
		if len(m.warnings) > 0 {
			lines = append(lines, m.paint(clean("Aviso: "+strings.Join(m.warnings, "; ")), "warning"))
		}
		if m.notice != "" {
			lines = append(lines, clean(strings.ReplaceAll(m.notice, "\n", " | ")))
		}
		room := max(1, m.height-len(lines)-2)
		if m.width >= 110 {
			leftWidth := m.width * 55 / 100
			left := m.listLines(leftWidth, room)
			right := m.detailLines(m.detailWidth())
			start := min(m.offset, max(0, len(right)-1))
			right = right[start:min(len(right), start+room)]
			separator := " │ "
			if m.options.ASCII {
				separator = " | "
			}
			for row := 0; row < max(len(left), len(right)); row++ {
				l, r := "", ""
				if row < len(left) {
					l = left[row]
				}
				if row < len(right) {
					r = right[row]
				}
				lines = append(lines, l+strings.Repeat(" ", max(0, leftWidth-ansi.StringWidth(l)))+separator+r)
			}
		} else if m.detailMode {
			body := m.detailLines(m.width)
			start := min(m.offset, max(0, len(body)-1))
			lines = append(lines, body[start:min(len(body), start+room)]...)
		} else {
			lines = append(lines, m.listLines(m.width, room)...)
		}
	}
	footer := "Tab foco | s/d/x accion | t transiciones | o abrir | ? ayuda | q salir"
	if m.filtering {
		footer = "Filtro local: escribe texto | Enter aceptar | Esc limpiar | Ctrl+U borrar"
	}
	lines = append(lines, clean(footer))
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}
