package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

func TestIssueJSONContractAndUnknownFields(t *testing.T) {
	r := app.SearchResult{Issues: []domain.Issue{{Ref: domain.IssueRef{ID: "1", Key: "APP-1"}, Status: domain.Status{Category: domain.CategoryUnknown}}}, Meta: app.ReadMeta{Profile: "work", FetchedAt: time.Now(), Source: "network"}, Warnings: []string{}}
	e := SearchEnvelope(r, &domain.Error{Kind: domain.Partial, Message: "incomplete"})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{`"ok":false`, `"complete":false`, `"returned":1`, `"updated_at":null`, `"due_date":null`, `"resolution":null`, `"category":"unknown"`, `"code":"partial_result"`} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
	if strings.Contains(text, `"Ref"`) || strings.Contains(text, `"Summary"`) {
		t.Fatal("domain structs leaked")
	}
}
func TestDetailRenderingIncludesLinksAndSections(t *testing.T) {
	d := domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "1", Key: "APP-1"}, Summary: "safe\x1b]52;c;payload\x07 text"}, Description: []domain.Block{{Kind: "paragraph", Children: []domain.Block{{Kind: "text", Text: "link", URL: "https://example.com"}}}}, Comments: &domain.CommentPage{Items: []domain.Comment{{ID: "c", Body: []domain.Block{{Kind: "text", Text: "body"}}}}}, History: &domain.HistoryPage{Items: []domain.HistoryEntry{{ID: "h", Changes: []domain.Change{{Field: "status", From: "Todo", To: "Done"}}}}}}
	text := DetailText(app.DetailResult{Detail: d})
	for _, want := range []string{"safe text", "https://example.com", "Comments:", "body", "History:", "Todo -> Done", "Incomplete"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
	if strings.Contains(text, "\x1b") || strings.Contains(text, "payload") {
		t.Fatal(text)
	}
}
