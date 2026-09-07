package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"time"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type IdentityClient interface {
	Myself(context.Context, config.Profile, ports.Secret) (domain.User, error)
}
type Access struct {
	Path    string
	Env     func(string) string
	Secrets ports.SecretStore
	Jira    IdentityClient
}
type Identity struct {
	Profile     string `json:"profile"`
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Stored      bool   `json:"credential_stored"`
}
type ProfileInfo struct {
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	SiteURL string `json:"site_url"`
	Method  string `json:"method"`
}
type AuthStatus struct {
	Profile   string `json:"profile"`
	Source    string `json:"credential_source"`
	Available bool   `json:"credential_available"`
	Verified  bool   `json:"verified"`
}

func problem(kind domain.ErrorKind, s string) error { return &domain.Error{Kind: kind, Message: s} }
func (a Access) selected(name, email string) (string, config.Profile, error) {
	c, err := config.Load(a.Path)
	if err != nil {
		return "", config.Profile{}, err
	}
	n, p, err := c.Select(name, a.Env)
	if err != nil {
		return n, p, err
	}
	if email != "" {
		p.Auth.Email = email
	}
	return n, p, p.Validate()
}
func (a Access) Login(ctx context.Context, name string, p config.Profile, token ports.Secret, persist bool) (Identity, error) {
	if !config.ValidName(name) {
		return Identity{}, problem(domain.InvalidInput, "El perfil requiere un nombre de hasta 64 letras, números, guiones o guiones bajos.")
	}
	if err := p.Validate(); err != nil {
		return Identity{}, err
	}
	// Do not touch disk or keyring until Jira confirms this exact identity.
	user, err := a.Jira.Myself(ctx, p, token)
	if err != nil {
		return Identity{}, err
	}
	p.AccountID = user.ID
	p.Auth.CredentialRef = ""
	if persist {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return Identity{}, problem(domain.Internal, "No se pudo generar la referencia de credencial.")
		}
		p.Auth.CredentialRef = hex.EncodeToString(b[:])
	}
	stored := false
	var previousRef string
	err = config.Update(ctx, a.Path, func(c *config.Config) error {
		old := c.Profiles[name]
		previousRef = old.Auth.CredentialRef
		if p.DefaultProject == "" {
			p.DefaultProject = old.DefaultProject
		}
		if persist {
			if err := a.Secrets.Set(ctx, ports.CredentialRef(p.Auth.CredentialRef), token); err != nil {
				return err
			}
			stored = true
		}
		p.RetiredCredentialRefs = append([]string(nil), old.RetiredCredentialRefs...)
		if previousRef != "" {
			p.RetiredCredentialRefs = append(p.RetiredCredentialRefs, previousRef)
		}
		c.Profiles[name] = p
		c.ActiveProfile = name
		return nil
	})
	if err != nil {
		if stored {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = a.Secrets.Delete(cleanup, ports.CredentialRef(p.Auth.CredentialRef))
		}
		return Identity{}, err
	}
	// Keep the old credential until the new configuration is durable. On a
	// cleanup failure keep its opaque reference so logout can retry removal.
	if previousRef != "" {
		if err := a.Secrets.Delete(ctx, ports.CredentialRef(previousRef)); err != nil {
			return Identity{}, problem(domain.Authentication, "El perfil se guardó, pero no se pudo eliminar la credencial anterior. Desbloquea el llavero y ejecuta auth logout para reintentar.")
		}
		if err := config.Update(ctx, a.Path, func(c *config.Config) error {
			p, ok := c.Profiles[name]
			if !ok {
				return nil
			}
			refs := p.RetiredCredentialRefs[:0]
			for _, r := range p.RetiredCredentialRefs {
				if r != previousRef {
					refs = append(refs, r)
				}
			}
			p.RetiredCredentialRefs = refs
			c.Profiles[name] = p
			return nil
		}); err != nil {
			return Identity{}, err
		}
	}
	return Identity{name, user.ID, user.DisplayName, persist}, nil
}
func (a Access) credential(ctx context.Context, p config.Profile, explicit ports.Secret) (ports.Secret, string, error) {
	if explicit.Reveal() != "" {
		return explicit, "stdin", nil
	}
	if v := a.Env("JFLOW_TOKEN"); v != "" {
		return ports.NewSecret(v), "environment", nil
	}
	if p.Auth.CredentialRef == "" {
		return ports.Secret{}, "none", problem(domain.Authentication, "No hay credencial guardada; usa JFLOW_TOKEN o --token-stdin.")
	}
	s, err := a.Secrets.Get(ctx, ports.CredentialRef(p.Auth.CredentialRef))
	return s, "keyring", err
}
func (a Access) Me(ctx context.Context, name, email string, token ports.Secret) (Identity, error) {
	n, p, err := a.selected(name, email)
	if err != nil {
		return Identity{}, err
	}
	s, _, err := a.credential(ctx, p, token)
	if err != nil {
		return Identity{}, err
	}
	user, err := a.Jira.Myself(ctx, p, s)
	if err != nil {
		return Identity{}, err
	}
	if p.AccountID != "" && p.AccountID != user.ID {
		return Identity{}, problem(domain.Authentication, "La credencial corresponde a otra identidad; ejecuta auth login para actualizar el perfil.")
	}
	return Identity{n, user.ID, user.DisplayName, p.Auth.CredentialRef != ""}, nil
}
func (a Access) Status(ctx context.Context, name, email string) (AuthStatus, error) {
	n, p, err := a.selected(name, email)
	if err != nil {
		return AuthStatus{}, err
	}
	_, source, err := a.credential(ctx, p, ports.Secret{})
	if err != nil {
		return AuthStatus{}, err
	}
	return AuthStatus{n, source, true, false}, nil
}
func (a Access) Profiles() ([]ProfileInfo, error) {
	c, err := config.Load(a.Path)
	if err != nil {
		return nil, err
	}
	result := []ProfileInfo{}
	for n, p := range c.Profiles {
		result = append(result, ProfileInfo{n, n == c.ActiveProfile, p.SiteURL, p.Auth.Method})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
func (a Access) Use(ctx context.Context, name string) error {
	return config.Update(ctx, a.Path, func(c *config.Config) error {
		if _, ok := c.Profiles[name]; !ok {
			return problem(domain.InvalidInput, "El perfil no existe.")
		}
		c.ActiveProfile = name
		return nil
	})
}
func (a Access) Logout(ctx context.Context, name string) (map[string]any, error) {
	selected := ""
	err := config.Update(ctx, a.Path, func(c *config.Config) error {
		n, p, err := c.Select(name, a.Env)
		if err != nil {
			return err
		}
		selected = n
		if p.Auth.CredentialRef != "" {
			if err := a.Secrets.Delete(ctx, ports.CredentialRef(p.Auth.CredentialRef)); err != nil {
				return err
			}
		}
		for _, ref := range p.RetiredCredentialRefs {
			if err := a.Secrets.Delete(ctx, ports.CredentialRef(ref)); err != nil {
				return err
			}
		}
		// F1 creates no issue cache; removing the profile purges local private identity.
		delete(c.Profiles, n)
		if c.ActiveProfile == n {
			c.ActiveProfile = ""
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"profile": selected, "local_credentials_removed": true, "environment_token_present": a.Env("JFLOW_TOKEN") != "", "remote_token_revoked": false}, nil
}
