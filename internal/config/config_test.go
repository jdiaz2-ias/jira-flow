package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixture() Profile {
	return Profile{Provider: "jira-cloud", SiteURL: "https://example.atlassian.net", Auth: Auth{Method: "api-token-unscoped", Email: "user@example.com"}}
}
func TestPaths(t *testing.T) {
	env := func(k string) string {
		return map[string]string{"XDG_CONFIG_HOME": "/cfg", "XDG_CACHE_HOME": "/cache", "XDG_STATE_HOME": "/state"}[k]
	}
	linux := ResolvePaths("linux", "/home/u", env)
	if linux.Config != "/cfg/jflow/config.json" || linux.State != "/state/jflow" || linux.Cache != "/cache/jflow" {
		t.Fatal(linux)
	}
	mac := ResolvePaths("darwin", "/Users/u", env)
	if mac.Config != "/Users/u/Library/Application Support/jflow/config.json" || mac.Cache != "/Users/u/Library/Caches/jflow" {
		t.Fatal(mac)
	}
	if ResolvePaths("darwin", "/u", func(k string) string {
		if k == "JFLOW_CONFIG" {
			return "/override.json"
		}
		return ""
	}).Config != "/override.json" {
		t.Fatal("override")
	}
}
func TestAtomicConcurrentUpdatesAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config.json")
	var wg sync.WaitGroup
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Update(context.Background(), path, func(c *Config) error { c.Profiles[name] = fixture(); return nil }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	c, err := Load(path)
	if err != nil || len(c.Profiles) != 5 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, tc := range []struct {
		path string
		mode os.FileMode
	}{{path, 0600}, {filepath.Dir(path), 0700}} {
		info, e := os.Stat(tc.path)
		if e != nil || info.Mode().Perm() != tc.mode {
			t.Fatalf("private permissions %v %v", info, e)
		}
	}
	before, _ := os.ReadFile(path)
	err = Update(context.Background(), path, func(c *Config) error { c.SchemaVersion = 999; return nil })
	after, _ := os.ReadFile(path)
	if err == nil || string(before) != string(after) {
		t.Fatal("failed update changed file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Update(ctx, path, func(c *Config) error { t.Fatal("canceled update executed"); return nil }) == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestRejectInvalidConfigurationAndURLs(t *testing.T) {
	for _, raw := range []string{`{"schema_version":9,"profiles":{}}`, `{"schema_version":0,"profiles":{}}`, `{"profiles":{}}`, `{"schema_version":1,"profiles":{},"token":"synthetic"}`, `{"schema_version":1,"profiles":{}} {}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, []byte(raw), 0600)
		if _, e := Load(path); e == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, site := range []string{"http://example.atlassian.net", "https://example.atlassian.net.evil.com", "https://u:p@example.atlassian.net", "https://example.atlassian.net/path", "https://example.atlassian.net/?x=1", "https://example.atlassian.net:443", "https://example.atlassian.net/#x"} {
		p := fixture()
		p.SiteURL = site
		if _, e := p.BaseURL(); e == nil {
			t.Errorf("accepted %s", site)
		}
	}
	p := fixture()
	p.Auth.Method = "api-token-scoped"
	p.CloudID = "cloud-123"
	base, e := p.BaseURL()
	if e != nil || base != "https://api.atlassian.com/ex/jira/cloud-123" {
		t.Fatal(base, e)
	}
	p.CloudID = "../bad"
	if _, e = p.BaseURL(); e == nil {
		t.Fatal("cloud path traversal")
	}
}
func TestProfilePrecedenceAndIsolation(t *testing.T) {
	c := Config{SchemaVersion: 1, ActiveProfile: "a", Profiles: map[string]Profile{"a": fixture(), "b": fixture()}}
	env := func(k string) string {
		return map[string]string{"JFLOW_PROFILE": "b", "JFLOW_EMAIL": "env@example.com"}[k]
	}
	n, p, e := c.Select("a", env)
	if e != nil || n != "a" || p.Auth.Email != "env@example.com" {
		t.Fatal(n, p, e)
	}
	n, _, _ = c.Select("", env)
	if n != "b" || c.ActiveProfile != "a" || c.Profiles["a"].Auth.Email != "user@example.com" {
		t.Fatal("selection mutated config")
	}
}
func TestLockWaitHonorsContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Update(context.Background(), path, func(c *Config) error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := Update(ctx, path, func(c *Config) error { t.Error("lock bypassed"); return nil })
	close(release)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	if !json.Valid(b) {
		t.Fatal("invalid final JSON")
	}
}
