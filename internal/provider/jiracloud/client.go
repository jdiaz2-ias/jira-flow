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
			return nil, &domain.Error{Kind: domain.InvalidInput, Message: "No se pudo leer la CA corporativa."}
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, &domain.Error{Kind: domain.InvalidInput, Message: "CA corporativa inválida."}
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
		return user, failure(domain.Authentication, "Falta el token. Ejecuta auth login o proporciona JFLOW_TOKEN.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rest/api/3/myself", nil)
	if err != nil {
		return user, failure(domain.InvalidInput, "No se pudo preparar la solicitud.")
	}
	req.SetBasicAuth(p.Auth.Email, secret.Reveal())
	req.Header.Set("Accept", "application/json")
	// Copy the client so callers cannot accidentally enable authenticated redirects.
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return user, failure(domain.Canceled, "Operación cancelada.")
		}
		return user, failure(domain.Unavailable, "No se pudo conectar con Jira; comprueba red, proxy y certificados.")
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 401:
		return user, failure(domain.Authentication, "Jira rechazó la credencial; comprueba correo, vencimiento o revocación del token.")
	case resp.StatusCode == 403:
		return user, failure(domain.Forbidden, "Jira denegó el acceso; comprueba permisos y scopes.")
	case resp.StatusCode == 404:
		return user, failure(domain.NotFound, "No se encontró el sitio o el endpoint Jira.")
	case resp.StatusCode == 429 || resp.StatusCode >= 500:
		return user, failure(domain.Unavailable, "Jira no está disponible o limitó las solicitudes; intenta más tarde.")
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return user, failure(domain.Forbidden, "Se rechazó una redirección autenticada de Jira.")
	case resp.StatusCode != 200:
		return user, failure(domain.Unavailable, "Jira devolvió una respuesta inesperada.")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return user, failure(domain.Canceled, "Operación cancelada.")
		}
		return user, failure(domain.Unavailable, "No se pudo leer la respuesta Jira.")
	}
	if len(b) > 1024*1024 {
		return user, failure(domain.Unavailable, "La respuesta Jira supera el límite permitido.")
	}
	var wire struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		Active      *bool  `json:"active"`
	}
	if json.Unmarshal(b, &wire) != nil || wire.AccountID == "" {
		return user, failure(domain.Unavailable, "Respuesta de identidad Jira inválida.")
	}
	if wire.Active != nil && !*wire.Active {
		return user, failure(domain.Authentication, "La cuenta Jira está inactiva.")
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
