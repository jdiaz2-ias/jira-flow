package jiracloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

func TestTransitionMetadataUsesLiveIDsAndSchema(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/rest/api/3/issue/APP-123/transitions" || r.URL.Query().Get("expand") != "transitions.fields" {
			t.Error(r.Method, r.URL)
		}
		w.Write(fixtureBytes(t, "transitions.json"))
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	ts, e := s.ListTransitions(context.Background(), domain.IssueRef{Key: "APP-123"})
	if e != nil || len(ts) != 2 || ts[1].ID != "31" || ts[1].Fields[0].Type != "resolution" || !ts[1].Fields[0].Required || ts[1].Fields[0].AllowedValues[0].ID != "10000" {
		t.Fatal(ts, e)
	}
}
func TestWritePayloadIsMinimalScopedAndNeverRetried(t *testing.T) {
	for _, status := range []int{204, 400, 401, 403, 404, 409, 429, 500, 503, 302, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/ex/jira/cloud-123/rest/api/3/issue/APP-123/transitions" {
					t.Error(r.Method, r.URL)
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 2 || body["transition"].(map[string]any)["id"] != "31" {
					t.Error(body)
				}
				fields := body["fields"].(map[string]any)
				if len(fields) != 1 || fields["resolution"].(map[string]any)["id"] != "10000" {
					t.Error(fields)
				}
				if r.Header.Get("Idempotency-Key") != "" {
					t.Error("invented idempotency")
				}
				w.Header().Set("Location", "https://evil.example")
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				if status != 204 {
					fmt.Fprint(w, `{"errors":{"resolution":"synthetic-token invalid resolution"},"errorMessages":["synthetic-token validator"]}`)
				}
			})
			s := &Session{client, profile("api-token-scoped"), ports.NewSecret("synthetic-token")}
			r, e := s.ApplyTransition(context.Background(), domain.TransitionRequest{Issue: domain.IssueRef{Key: "APP-123"}, TransitionID: "31", Fields: map[string]any{"resolution": map[string]any{"id": "10000"}}})
			if calls != 1 {
				t.Fatal("write retried", calls)
			}
			switch {
			case status == 204:
				if e != nil || r.State != domain.ApplyAcceptedUnverified {
					t.Fatal(r, e)
				}
			case status >= 500 || status == 200:
				if domain.ExitCode(e) != 9 || r.State != domain.ApplyUnknown {
					t.Fatal(r, e)
				}
			default:
				if e == nil || r.State != domain.ApplyFailed {
					t.Fatal(r, e)
				}
			}
			if e != nil {
				b, _ := json.Marshal(e.(*domain.Error).Details)
				if strings.Contains(string(b)+e.Error(), "synthetic-token") {
					t.Fatal("secret exposed")
				}
			}
		})
	}
}
func TestLostWriteResponseStaysUnknownAndIsNotResent(t *testing.T) {
	var calls atomic.Int32
	received := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		calls.Add(1)
		select {
		case received <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		select {
		case <-received:
			cancel()
		case <-ctx.Done():
		}
	}()
	r, e := s.ApplyTransition(ctx, domain.TransitionRequest{Issue: domain.IssueRef{Key: "APP-1"}, TransitionID: "t-1"})
	if domain.ExitCode(e) != 9 || r.State != domain.ApplyUnknown || calls.Load() != 1 {
		t.Fatal(r, e, calls.Load())
	}
}
func TestRetryingTransportRejectsMutations(t *testing.T) {
	client := setup(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected network") })
	_, e := client.read(context.Background(), profile("api-token-unscoped"), ports.NewSecret("token"), http.MethodPost, "/rest/api/3/issue/APP-1/transitions", nil)
	if domain.ExitCode(e) != 2 {
		t.Fatal(e)
	}
}
func TestVerificationRequestsAndReturnsSelectedFieldValues(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("fields"), "customfield_123") {
			t.Error(r.URL)
		}
		fmt.Fprint(w, `{"id":"1","key":"APP-1","fields":{"status":{"id":"5","name":"Resolved","statusCategory":{"key":"done"}},"customfield_123":{"id":"option-real","value":"Chosen"}}}`)
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	d, e := s.GetIssue(context.Background(), domain.IssueRef{Key: "APP-1"}, domain.DetailOptions{Fields: []string{"customfield_123"}})
	if e != nil || d.Values["customfield_123"].(map[string]any)["id"] != "option-real" {
		t.Fatal(d, e)
	}
}
func TestMissingTransitionFieldsAreNotAssumedEmpty(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"transitions":[{"id":"1","name":"Move","to":{"id":"2"}}]}`)
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	if _, e := s.ListTransitions(context.Background(), domain.IssueRef{Key: "APP-1"}); e == nil {
		t.Fatal("missing metadata accepted")
	}
}
