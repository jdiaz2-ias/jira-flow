package jiracloud

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

// Route validated production URLs to a TLS fixture while preserving request paths.
type route struct {
	target    *url.URL
	transport http.RoundTripper
}

func (r route) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = r.target.Scheme
	u.Host = r.target.Host
	clone.URL = &u
	return r.transport.RoundTrip(clone)
}
func setup(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return &Client{HTTP: &http.Client{Transport: route{u, srv.Client().Transport}, Timeout: time.Second}}
}
func profile(method string) config.Profile {
	return config.Profile{Provider: "jira-cloud", SiteURL: "https://example.atlassian.net", CloudID: "cloud-123", Auth: config.Auth{Method: method, Email: "user@example.com"}}
}
func TestBothTokenMethods(t *testing.T) {
	for _, method := range []string{"api-token-scoped", "api-token-unscoped"} {
		t.Run(method, func(t *testing.T) {
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				want := "/rest/api/3/myself"
				if method == "api-token-scoped" {
					want = "/ex/jira/cloud-123" + want
				}
				if r.URL.Path != want {
					t.Errorf("path %s", r.URL.Path)
				}
				email, token, ok := r.BasicAuth()
				if !ok || email != "user@example.com" || token != "synthetic-token" {
					t.Error("bad authorization")
				}
				fmt.Fprint(w, `{"accountId":"u-1","displayName":"Synthetic User","active":true}`)
			})
			u, e := client.Myself(context.Background(), profile(method), ports.NewSecret("synthetic-token"))
			if e != nil || u.ID != "u-1" {
				t.Fatal(u, e)
			}
		})
	}
}
func TestErrorsAreRedacted(t *testing.T) {
	for _, tc := range []struct {
		status int
		kind   domain.ErrorKind
	}{{401, domain.Authentication}, {403, domain.Forbidden}, {404, domain.NotFound}, {429, domain.Unavailable}, {500, domain.Unavailable}, {302, domain.Forbidden}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			calls := 0
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "https://evil.example/leak")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, "synthetic-secret "+r.Header.Get("Authorization"))
			})
			_, err := client.Myself(context.Background(), profile("api-token-unscoped"), ports.NewSecret("synthetic-secret"))
			e, ok := err.(*domain.Error)
			if !ok || e.Kind != tc.kind || calls != 1 || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatalf("%v calls=%d", err, calls)
			}
		})
	}
}
func TestInvalidOversizeAndSanitizedResponses(t *testing.T) {
	for _, body := range []string{`{}`, `{"accountId":"u"} {}`, strings.Repeat("x", 1024*1024+1), `{"accountId":"u","active":false}`} {
		client := setup(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		if _, e := client.Myself(context.Background(), profile("api-token-unscoped"), ports.NewSecret("secret")); e == nil {
			t.Fatal("accepted invalid body")
		}
	}
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"accountId":"u","displayName":"\u001b[31msecret"}`)
	})
	u, e := client.Myself(context.Background(), profile("api-token-unscoped"), ports.NewSecret("secret"))
	if e != nil || strings.ContainsAny(u.DisplayName, "\x1b") || strings.Contains(u.DisplayName, "secret") {
		t.Fatal(u, e)
	}
}
func TestCanceledRequestAndNoCredentialLeak(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Myself(ctx, profile("api-token-unscoped"), ports.NewSecret("secret"))
	if e, ok := err.(*domain.Error); !ok || e.Kind != domain.Canceled {
		t.Fatal(err)
	}
}

func TestAuthorizationEchoIsRedacted(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"accountId":"u","displayName":%q}`, r.Header.Get("Authorization"))
	})
	u, e := client.Myself(context.Background(), profile("api-token-unscoped"), ports.NewSecret("synthetic-secret"))
	if e != nil || u.DisplayName != "[REDACTED]" {
		t.Fatal(u, e)
	}
}
