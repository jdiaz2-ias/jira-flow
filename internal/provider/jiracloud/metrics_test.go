package jiracloud

import (
	"context"
	"fmt"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
	"net/http"
	"strings"
	"testing"
)

func TestTimeFieldsPreserveZeroMissingAndInvalid(t *testing.T) {
	for _, timeFields := range []string{`"timeSpentSeconds":0,"remainingEstimateSeconds":0,"originalEstimateSeconds":3600`, `"timeSpentSeconds":-1`, `"remainingEstimateSeconds":null`} {
		t.Run(timeFields, func(t *testing.T) {
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Query().Get("fields"), "timetracking") {
					t.Error("time not requested")
				}
				fmt.Fprintf(w, `{"id":"1","key":"APP-1","fields":{"timetracking":{%s}}}`, timeFields)
			})
			s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("synthetic-token")}
			d, err := s.GetIssue(context.Background(), domain.IssueRef{Key: "APP-1"}, domain.DetailOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(timeFields, "3600") {
				if d.Time.Spent == nil || *d.Time.Spent != 0 || d.Time.Remaining == nil || *d.Time.Original != 3600 {
					t.Fatal(d.Time)
				}
			} else if d.Time.Spent != nil || d.Time.Original != nil || d.Time.Remaining != nil {
				t.Fatal(d.Time)
			}
		})
	}
}
