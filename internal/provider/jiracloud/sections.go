package jiracloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"jira-flow.local/jflow/internal/domain"
)

func readSection[T any](ctx context.Context, s *Session, key, section string, options domain.DetailOptions, start int, decode func(json.RawMessage) (T, error)) ([]T, domain.PageInfo, error) {
	items := []T{}
	info := domain.PageInfo{StartAt: start}
	size := options.PageSize
	if size == 0 {
		size = 50
	}
	limit := options.SectionLimit
	if limit == 0 {
		limit = 50
		if options.All {
			limit = 5000
		}
	}
	if start < 0 || size < 1 || size > 100 || limit < 1 || limit > 5000 {
		return items, info, failure(domain.InvalidInput, "Límites de sección inválidos.")
	}
	seen := map[string]bool{}
	offset := start
	for requests := 0; requests < 1000; requests++ {
		remaining := limit - len(items)
		requestSize := min(size, remaining)
		path := "/rest/api/3/issue/" + key + "/" + section + "?" + url.Values{"startAt": {strconv.Itoa(offset)}, "maxResults": {strconv.Itoa(requestSize)}}.Encode()
		b, err := s.Client.read(ctx, s.Profile, s.Secret, http.MethodGet, path, nil)
		if err != nil {
			info.NextStart = &offset
			return items, info, err
		}
		var page struct {
			Start    *int              `json:"startAt"`
			Total    *int              `json:"total"`
			Last     *bool             `json:"isLast"`
			Comments []json.RawMessage `json:"comments"`
			Values   []json.RawMessage `json:"values"`
		}
		if json.Unmarshal(b, &page) != nil || page.Start == nil || *page.Start != offset || (page.Total != nil && *page.Total < 0) {
			return items, info, failure(domain.Unavailable, "Paginación de sección Jira inválida.")
		}
		entries := page.Values
		if section == "comment" {
			entries = page.Comments
		}
		if entries == nil || len(entries) > 100 {
			return items, info, failure(domain.Unavailable, "Sección Jira inválida.")
		}
		info.Total = page.Total
		consumed := 0
		added := 0
		for _, raw := range entries {
			if len(items) >= limit {
				break
			}
			consumed++
			var id struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &id) != nil || id.ID == "" {
				return items, info, failure(domain.Unavailable, "Registro de sección Jira inválido.")
			}
			if seen[id.ID] {
				continue
			}
			value, err := decode(raw)
			if err != nil {
				return items, info, err
			}
			seen[id.ID] = true
			items = append(items, value)
			added++
		}
		offset += consumed
		last := page.Last != nil && *page.Last
		if page.Last == nil && page.Total != nil {
			last = offset >= *page.Total
		}
		if consumed == len(entries) && last {
			info.Complete = true
			info.NextStart = nil
			return items, info, nil
		}
		next := offset
		info.NextStart = &next
		if len(items) >= limit {
			if options.All {
				return items, info, &domain.Error{Kind: domain.Partial, Message: "Se alcanzó el límite protector de la sección."}
			}
			return items, info, nil
		}
		if consumed == 0 || added == 0 {
			return items, info, failure(domain.Unavailable, "La paginación de la sección no avanzó.")
		}
	}
	return items, info, &domain.Error{Kind: domain.Partial, Message: "Se alcanzó el límite protector de páginas de la sección."}
}
func (s *Session) comments(ctx context.Context, key string, o domain.DetailOptions) (domain.CommentPage, error) {
	clean := cleanRemote(s.Profile, s.Secret)
	items, info, err := readSection(ctx, s, key, "comment", o, o.CommentsStart, func(raw json.RawMessage) (domain.Comment, error) {
		var w struct {
			ID      string          `json:"id"`
			Author  *wireUser       `json:"author"`
			Body    json.RawMessage `json:"body"`
			Created string          `json:"created"`
		}
		if json.Unmarshal(raw, &w) != nil {
			return domain.Comment{}, failure(domain.Unavailable, "Comentario Jira inválido.")
		}
		blocks, _ := normalizeADF(w.Body, clean)
		return domain.Comment{ID: clean(w.ID), Author: user(w.Author, clean), Body: blocks, CreatedAt: parseTime(w.Created)}, nil
	})
	return domain.CommentPage{Items: items, PageInfo: info}, err
}
func (s *Session) history(ctx context.Context, key string, o domain.DetailOptions) (domain.HistoryPage, error) {
	clean := cleanRemote(s.Profile, s.Secret)
	items, info, err := readSection(ctx, s, key, "changelog", o, o.HistoryStart, func(raw json.RawMessage) (domain.HistoryEntry, error) {
		var w struct {
			ID      string    `json:"id"`
			Author  *wireUser `json:"author"`
			Created string    `json:"created"`
			Items   []struct {
				Field string `json:"field"`
				From  string `json:"fromString"`
				To    string `json:"toString"`
			} `json:"items"`
		}
		if json.Unmarshal(raw, &w) != nil {
			return domain.HistoryEntry{}, failure(domain.Unavailable, "Historial Jira inválido.")
		}
		result := domain.HistoryEntry{ID: clean(w.ID), Author: user(w.Author, clean), CreatedAt: parseTime(w.Created), Changes: []domain.Change{}}
		for _, item := range w.Items {
			result.Changes = append(result.Changes, domain.Change{Field: clean(item.Field), From: clean(item.From), To: clean(item.To)})
		}
		return result, nil
	})
	return domain.HistoryPage{Items: items, PageInfo: info}, err
}
