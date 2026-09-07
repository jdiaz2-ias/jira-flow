// Package jiracloud implements Jira Cloud wire contracts.
package jiracloud

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type Client struct {
	HTTP  *http.Client
	Sleep func(context.Context, time.Duration) error
}

func New(caFile string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Could not read the corporate CA."}
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Invalid corporate CA."}
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &Client{HTTP: &http.Client{Transport: transport, Timeout: 30 * time.Second}}, nil
}
func failure(kind domain.ErrorKind, message string) error {
	return &domain.Error{Kind: kind, Message: message, Retryable: kind == domain.Unavailable}
}
func (c *Client) Myself(ctx context.Context, p config.Profile, secret ports.Secret) (domain.User, error) {
	var user domain.User
	if err := p.Validate(); err != nil {
		return user, err
	}
	base, err := p.BaseURL()
	if err != nil {
		return user, err
	}
	if secret.Reveal() == "" {
		return user, failure(domain.Authentication, "Token missing. Run auth login or provide JFLOW_TOKEN.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rest/api/3/myself", nil)
	if err != nil {
		return user, failure(domain.InvalidInput, "Could not prepare the request.")
	}
	req.SetBasicAuth(p.Auth.Email, secret.Reveal())
	req.Header.Set("Accept", "application/json")
	// Copy the client so callers cannot accidentally enable authenticated redirects.
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return user, failure(domain.Canceled, "Operation canceled.")
		}
		return user, failure(domain.Unavailable, "Could not connect to Jira; check network, proxy, and certificates.")
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 401:
		return user, failure(domain.Authentication, "Jira rejected the credential; check email, expiration, or token revocation.")
	case resp.StatusCode == 403:
		return user, failure(domain.Forbidden, "Jira denied access; check permissions and scopes.")
	case resp.StatusCode == 404:
		return user, failure(domain.NotFound, "The Jira site or endpoint was not found.")
	case resp.StatusCode == 429 || resp.StatusCode >= 500:
		return user, failure(domain.Unavailable, "Jira is unavailable or rate-limited; try again later.")
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return user, failure(domain.Forbidden, "An authenticated redirect from Jira was rejected.")
	case resp.StatusCode != 200:
		return user, failure(domain.Unavailable, "Jira returned an unexpected response.")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return user, failure(domain.Canceled, "Operation canceled.")
		}
		return user, failure(domain.Unavailable, "Could not read the Jira response.")
	}
	if len(b) > 1024*1024 {
		return user, failure(domain.Unavailable, "The Jira response exceeds the allowed limit.")
	}
	var wire struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		Active      *bool  `json:"active"`
	}
	if json.Unmarshal(b, &wire) != nil || wire.AccountID == "" {
		return user, failure(domain.Unavailable, "Invalid Jira identity response.")
	}
	if wire.Active != nil && !*wire.Active {
		return user, failure(domain.Authentication, "The Jira account is inactive.")
	}
	clean := func(s string) string {
		s = strings.ReplaceAll(s, req.Header.Get("Authorization"), "[REDACTED]")
		s = strings.ReplaceAll(s, strings.TrimPrefix(req.Header.Get("Authorization"), "Basic "), "[REDACTED]")
		s = strings.ReplaceAll(s, secret.Reveal(), "[REDACTED]")
		return strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return -1
			}
			return r
		}, s)
	}
	return domain.User{ID: clean(wire.AccountID), DisplayName: clean(wire.DisplayName)}, nil
}
