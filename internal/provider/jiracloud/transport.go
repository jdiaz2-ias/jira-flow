package jiracloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

func (c *Client) read(ctx context.Context, p config.Profile, secret ports.Secret, method, path string, body any) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	base, err := p.BaseURL()
	if err != nil {
		return nil, err
	}
	if secret.Reveal() == "" {
		return nil, failure(domain.Authentication, "Jira credential is missing.")
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, failure(domain.InvalidInput, "Invalid request.")
		}
	}
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
		if err != nil {
			return nil, failure(domain.InvalidInput, "Invalid request.")
		}
		req.SetBasicAuth(p.Auth.Email, secret.Reveal())
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, failure(domain.Canceled, "Operation cancelled or deadline exceeded.")
			}
			if attempt == 2 {
				return nil, failure(domain.Unavailable, "Could not connect to Jira; check network, proxy and certificates.")
			}
			if err = c.pause(ctx, "", attempt); err != nil {
				return nil, err
			}
			continue
		}
		retry := resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504
		if retry && attempt < 2 {
			delay := resp.Header.Get("Retry-After")
			resp.Body.Close()
			if err = c.pause(ctx, delay, attempt); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			switch {
			case resp.StatusCode == 400:
				return nil, failure(domain.InvalidInput, "Jira rejected the query or fields; check the JQL and filters.")
			case resp.StatusCode == 401:
				return nil, failure(domain.Authentication, "Jira rejected the credential; check token expiry or revocation.")
			case resp.StatusCode == 403:
				return nil, failure(domain.Forbidden, "Jira denied the read; check permissions and scopes.")
			case resp.StatusCode == 404:
				return nil, failure(domain.NotFound, "The issue does not exist or is not visible to this account.")
			case resp.StatusCode == 409:
				return nil, failure(domain.Conflict, "Jira detected a conflict; query again.")
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				return nil, failure(domain.Forbidden, "An authenticated Jira redirect was rejected.")
			default:
				return nil, failure(domain.Unavailable, "Jira is unavailable or has limited requests; try again later.")
			}
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
		resp.Body.Close()
		if err != nil {
			if ctx.Err() != nil {
				return nil, failure(domain.Canceled, "Operation cancelled or deadline exceeded.")
			}
			return nil, failure(domain.Unavailable, "Could not read Jira response.")
		}
		if len(b) > 4*1024*1024 {
			return nil, failure(domain.Unavailable, "The Jira response exceeds 4 MiB.")
		}
		if !json.Valid(b) {
			return nil, failure(domain.Unavailable, "Invalid JSON response; check whether a proxy or login page exists.")
		}
		return b, nil
	}
	return nil, failure(domain.Unavailable, "Could not complete the Jira read.")
}
func (c *Client) pause(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(100*(1<<attempt)+rand.IntN(100)) * time.Millisecond
	if retryAfter != "" {
		if secs, err := strconv.ParseInt(retryAfter, 10, 32); err == nil && secs >= 0 {
			delay = time.Duration(secs) * time.Second
		} else if when, err := http.ParseTime(retryAfter); err == nil {
			delay = time.Until(when)
			if delay < 0 {
				delay = 0
			}
		}
	}
	if deadline, ok := ctx.Deadline(); ok && delay >= time.Until(deadline) {
		return failure(domain.Unavailable, fmt.Sprintf("Jira asks to wait %d seconds; increase --timeout or retry later.", int64(delay.Seconds())+1))
	}
	if delay > 30*time.Second {
		return failure(domain.Unavailable, "Jira requires a long wait; retry later.")
	}
	if c.Sleep != nil {
		if err := c.Sleep(ctx, delay); err != nil {
			return failure(domain.Canceled, "Operation cancelled or deadline exceeded.")
		}
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return failure(domain.Canceled, "Operation cancelled or deadline exceeded.")
	case <-timer.C:
		return nil
	}
}
func cleanRemote(p config.Profile, token ports.Secret) func(string) string {
	basic := base64.StdEncoding.EncodeToString([]byte(p.Auth.Email + ":" + token.Reveal()))
	return func(s string) string {
		for _, sensitive := range []string{"Basic " + basic, basic, token.Reveal()} {
			if sensitive != "" {
				s = strings.ReplaceAll(s, sensitive, "[REDACTED]")
			}
		}
		return domain.CleanText(s, true)
	}
}
