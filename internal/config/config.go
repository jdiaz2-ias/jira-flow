// Package config owns private, versioned configuration and platform paths.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"jira-flow.local/jflow/internal/domain"
)

type Auth struct {
	Method        string `json:"method"`
	Email         string `json:"email"`
	CredentialRef string `json:"credential_ref,omitempty"`
}
type Profile struct {
	WorkflowRules         []domain.WorkflowRule `json:"workflow_rules,omitempty"`
	DefaultProject        string                `json:"default_project,omitempty"`
	RetiredCredentialRefs []string              `json:"retired_credential_refs,omitempty"`
	Provider              string                `json:"provider"`
	SiteURL               string                `json:"site_url"`
	CloudID               string                `json:"cloud_id,omitempty"`
	Auth                  Auth                  `json:"auth"`
	AccountID             string                `json:"account_id,omitempty"`
}
type Config struct {
	SchemaVersion int                `json:"schema_version"`
	ActiveProfile string             `json:"active_profile"`
	Email         string             `json:"email,omitempty"`
	Profiles      map[string]Profile `json:"profiles"`
}
type Paths struct{ Config, Cache, State string }

func ResolvePaths(platform, home string, env func(string) string) Paths {
	var p Paths
	if platform == "darwin" {
		p = Paths{filepath.Join(home, "Library", "Application Support", "jflow", "config.json"), filepath.Join(home, "Library", "Caches", "jflow"), filepath.Join(home, "Library", "Application Support", "jflow")}
	} else {
		base := func(key, fallback string) string {
			if v := env(key); filepath.IsAbs(v) {
				return v
			}
			return filepath.Join(home, fallback)
		}
		p = Paths{filepath.Join(base("XDG_CONFIG_HOME", ".config"), "jflow", "config.json"), filepath.Join(base("XDG_CACHE_HOME", ".cache"), "jflow"), filepath.Join(base("XDG_STATE_HOME", ".local/state"), "jflow")}
	}
	if v := env("JFLOW_CONFIG"); v != "" {
		p.Config = v
	}
	return p
}
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, invalid("Could not resolve home directory.")
	}
	return ResolvePaths(runtime.GOOS, home, os.Getenv), nil
}
func invalid(s string) error { return &domain.Error{Kind: domain.InvalidInput, Message: s} }

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var cloudPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,127}$`)
var refPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func ValidName(s string) bool { return namePattern.MatchString(s) }
func ValidRef(s string) bool  { return refPattern.MatchString(s) }
func (p Profile) BaseURL() (string, error) {
	u, err := url.Parse(p.SiteURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Port() != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" || !strings.HasSuffix(u.Hostname(), ".atlassian.net") || strings.ContainsAny(u.Hostname(), " /\\") {
		return "", invalid("Site must be a Jira Cloud HTTPS URL (*.atlassian.net), with no path or credentials.")
	}
	switch p.Auth.Method {
	case "api-token-unscoped":
		return strings.TrimSuffix(p.SiteURL, "/"), nil
	case "api-token-scoped":
		if !cloudPattern.MatchString(p.CloudID) {
			return "", invalid("Scoped token requires a valid explicit cloud ID.")
		}
		return "https://api.atlassian.com/ex/jira/" + p.CloudID, nil
	default:
		return "", invalid("Invalid method: use api-token-unscoped or api-token-scoped.")
	}
}
func (p Profile) Validate() error {
	seen := map[string]bool{}
	for _, rule := range p.WorkflowRules {
		if err := rule.Validate(); err != nil {
			return err
		}
		key := rule.ProjectID + "/" + rule.IssueTypeID + "/" + string(rule.Intent) + "/" + rule.FromStatusID
		if seen[key] {
			return invalid("Duplicate workflow rule for the same scope.")
		}
		seen[key] = true
	}

	if p.Provider != "jira-cloud" {
		return invalid("Unsupported provider; use jira-cloud.")
	}
	if _, err := p.BaseURL(); err != nil {
		return err
	}
	a, err := mail.ParseAddress(p.Auth.Email)
	if err != nil || a.Address != p.Auth.Email || strings.ContainsAny(p.Auth.Email, "\r\n:") {
		return invalid("A valid email is required.")
	}
	for _, ref := range p.RetiredCredentialRefs {
		if !ValidRef(ref) {
			return invalid("Invalid retired credential reference.")
		}
	}
	if p.Auth.CredentialRef != "" && !ValidRef(p.Auth.CredentialRef) {
		return invalid("Invalid credential reference.")
	}
	return nil
}
func (c Config) Validate() error {
	if c.SchemaVersion != 1 {
		return invalid("Unsupported configuration version; schema_version 1 is required.")
	}
	if c.Profiles == nil {
		return invalid("Configuration requires profiles.")
	}
	for name, p := range c.Profiles {
		if !ValidName(name) {
			return invalid("Invalid profile name.")
		}
		if p.Auth.Email == "" {
			p.Auth.Email = c.Email
		}
		if err := p.Validate(); err != nil {
			return err
		}
	}
	if c.ActiveProfile != "" {
		if _, ok := c.Profiles[c.ActiveProfile]; !ok {
			return invalid("Active profile does not exist.")
		}
	}
	return nil
}
func Load(path string) (Config, error) {
	c := Config{SchemaVersion: 1, Profiles: map[string]Profile{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, invalid("Could not read configuration.")
	}
	defer f.Close()
	c = Config{}
	dec := json.NewDecoder(io.LimitReader(f, 1024*1024+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, invalid("Invalid JSON configuration or unknown fields.")
	}
	if dec.Decode(new(any)) != io.EOF {
		return c, invalid("Configuration must contain a single JSON document.")
	}
	return c, c.Validate()
}

// Update locks the entire read/modify/write transaction. No schema predates v1:
// unknown versions are rejected without touching the original file.
func Update(ctx context.Context, path string, change func(*Config) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return invalid("Could not create configuration directory.")
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return invalid("Could not open configuration lock.")
	}
	defer lock.Close()
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return invalid("Could not lock configuration.")
		}
		select {
		case <-ctx.Done():
			return &domain.Error{Kind: domain.Canceled, Message: "Operation canceled."}
		case <-time.After(20 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if ctx.Err() != nil {
		return &domain.Error{Kind: domain.Canceled, Message: "Operation canceled."}
	}
	c, err := Load(path)
	if err != nil {
		return err
	}
	if err = change(&c); err != nil {
		return err
	}
	if err = c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return invalid("Could not serialize configuration.")
	}
	f, err := os.CreateTemp(dir, ".jflow-*")
	if err != nil {
		return invalid("Could not write configuration.")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	if err == nil {
		err = f.Close()
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return invalid("Could not save configuration.")
	}
	return nil
}
func (c Config) Select(flag string, env func(string) string) (string, Profile, error) {
	name := flag
	if name == "" {
		name = env("JFLOW_PROFILE")
	}
	if name == "" {
		name = c.ActiveProfile
	}
	p, ok := c.Profiles[name]
	if !ok {
		return "", Profile{}, invalid("Select an existing profile or run auth login.")
	}
	if p.Auth.Email == "" {
		p.Auth.Email = c.Email
	}
	if v := env("JFLOW_EMAIL"); v != "" {
		p.Auth.Email = v
	}
	return name, p, nil
}
