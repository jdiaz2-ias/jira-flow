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
		return nil, failure(domain.Authentication, "Falta la credencial Jira.")
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, failure(domain.InvalidInput, "Solicitud inválida.")
		}
	}
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
		if err != nil {
			return nil, failure(domain.InvalidInput, "Solicitud inválida.")
		}
		req.SetBasicAuth(p.Auth.Email, secret.Reveal())
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, failure(domain.Canceled, "Operación cancelada o plazo agotado.")
			}
			if attempt == 2 {
				return nil, failure(domain.Unavailable, "No se pudo conectar con Jira; comprueba red, proxy y certificados.")
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
				return nil, failure(domain.InvalidInput, "Jira rechazó la consulta o los campos; comprueba el JQL y los filtros.")
			case resp.StatusCode == 401:
				return nil, failure(domain.Authentication, "Jira rechazó la credencial; comprueba vencimiento o revocación del token.")
			case resp.StatusCode == 403:
				return nil, failure(domain.Forbidden, "Jira denegó la lectura; comprueba permisos y scopes.")
			case resp.StatusCode == 404:
				return nil, failure(domain.NotFound, "El issue no existe o no es visible para esta cuenta.")
			case resp.StatusCode == 409:
				return nil, failure(domain.Conflict, "Jira detectó un conflicto; vuelve a consultar.")
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				return nil, failure(domain.Forbidden, "Se rechazó una redirección autenticada de Jira.")
			default:
				return nil, failure(domain.Unavailable, "Jira no está disponible o limitó las solicitudes; intenta más tarde.")
			}
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
		resp.Body.Close()
		if err != nil {
			if ctx.Err() != nil {
				return nil, failure(domain.Canceled, "Operación cancelada o plazo agotado.")
			}
			return nil, failure(domain.Unavailable, "No se pudo leer la respuesta Jira.")
		}
		if len(b) > 4*1024*1024 {
			return nil, failure(domain.Unavailable, "La respuesta Jira supera 4 MiB.")
		}
		if !json.Valid(b) {
			return nil, failure(domain.Unavailable, "Respuesta JSON inválida; comprueba si existe un proxy o una página de login.")
		}
		return b, nil
	}
	return nil, failure(domain.Unavailable, "No se pudo completar la lectura Jira.")
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
		return failure(domain.Unavailable, fmt.Sprintf("Jira solicita esperar %d segundos; aumenta --timeout o reintenta después.", int64(delay.Seconds())+1))
	}
	if delay > 30*time.Second {
		return failure(domain.Unavailable, "Jira solicita una espera prolongada; reintenta más tarde.")
	}
	if c.Sleep != nil {
		if err := c.Sleep(ctx, delay); err != nil {
			return failure(domain.Canceled, "Operación cancelada o plazo agotado.")
		}
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return failure(domain.Canceled, "Operación cancelada o plazo agotado.")
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
