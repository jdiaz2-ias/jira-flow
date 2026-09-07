package jiracloud

import (
	"encoding/json"
	"net/url"

	"jira-flow.local/jflow/internal/domain"
)

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
	Attrs   struct {
		URL       string `json:"url"`
		Text      string `json:"text"`
		ShortName string `json:"shortName"`
	} `json:"attrs"`
	Marks []struct {
		Type  string `json:"type"`
		Attrs struct {
			Href string `json:"href"`
		} `json:"attrs"`
	} `json:"marks"`
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}
func normalizeADF(raw json.RawMessage, clean func(string) string) ([]domain.Block, []string) {
	empty := []domain.Block{}
	if len(raw) == 0 || string(raw) == "null" {
		return empty, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []domain.Block{{Kind: "text", Text: clean(text)}}, nil
	}
	var root adfNode
	if json.Unmarshal(raw, &root) != nil {
		return []domain.Block{{Kind: "unsupported", Text: "[contenido no soportado]"}}, []string{"La descripción contiene un formato no soportado."}
	}
	count := 0
	truncated := false
	var walk func(adfNode, int) domain.Block
	walk = func(n adfNode, depth int) domain.Block {
		count++
		if depth > 64 || count > 10000 {
			truncated = true
			return domain.Block{Kind: "unsupported", Text: "[contenido truncado]"}
		}
		b := domain.Block{Kind: n.Type, Text: clean(n.Text), Children: []domain.Block{}}
		switch n.Type {
		case "doc", "paragraph", "heading", "bulletList", "orderedList", "listItem", "codeBlock", "blockquote", "table", "tableRow", "tableCell", "tableHeader", "text":
		case "hardBreak":
			b.Text = "\n"
		case "rule":
			b.Text = "---"
		case "mention":
			b.Kind = "text"
			b.Text = clean(n.Attrs.Text)
			if b.Text == "" {
				b.Text = "[mención]"
			}
		case "emoji":
			b.Kind = "text"
			b.Text = clean(n.Attrs.ShortName)
		case "inlineCard", "blockCard":
			b.Kind = "link"
			b.URL = safeURL(clean(n.Attrs.URL))
			b.Text = b.URL
		default:
			b.Kind = "unsupported"
			if n.Text == "" && len(n.Content) == 0 {
				b.Text = "[contenido no soportado]"
			}
		}
		for _, mark := range n.Marks {
			if mark.Type == "link" {
				b.URL = safeURL(clean(mark.Attrs.Href))
			}
		}
		for _, child := range n.Content {
			if count >= 10000 {
				truncated = true
				break
			}
			b.Children = append(b.Children, walk(child, depth+1))
		}
		return b
	}
	blocks := []domain.Block{walk(root, 0)}
	warnings := []string{}
	if truncated {
		warnings = append(warnings, "El contenido ADF excede el límite de profundidad o nodos.")
	}
	return blocks, warnings
}
