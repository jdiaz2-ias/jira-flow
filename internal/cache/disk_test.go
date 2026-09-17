package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDiskAcrossInstancesTTLIsolationAndExpiry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Now().UTC()
	d := NewDisk(dir, "work")
	d.Now = func() time.Time { return now }
	if err := d.Put(ctx, "scope:detail:key", "detail", []byte(`{"safe":true}`), now, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	other := NewDisk(dir, "other")
	if b, _, _, err := other.Get(ctx, "scope:detail:key", true); err != nil || b != nil {
		t.Fatal(string(b), err)
	}
	second := NewDisk(dir, "work")
	second.Now = d.Now
	if b, _, stale, err := second.Get(ctx, "scope:detail:key", false); err != nil || b == nil || stale {
		t.Fatal(string(b), stale, err)
	}
	now = now.Add(31 * time.Second)
	if b, _, _, _ := second.Get(ctx, "scope:detail:key", false); b != nil {
		t.Fatal("expired online hit")
	}
	if b, _, stale, err := second.Get(ctx, "scope:detail:key", true); err != nil || b == nil || !stale {
		t.Fatal(stale, err)
	}
	now = now.Add(7 * 24 * time.Hour)
	if b, _, _, _ := second.Get(ctx, "scope:detail:key", true); b != nil {
		t.Fatal("expired offline hit")
	}
	s, err := second.Status(ctx)
	if err != nil || s.Entries != 0 {
		t.Fatal(s, err)
	}
}
func TestDiskPermissionsCorruptionSymlinkAndClear(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d := NewDisk(dir, "work")
	other := NewDisk(dir, "other")
	for _, x := range []*Disk{d, other} {
		if err := x.Put(ctx, "key", "search", []byte(`[]`), time.Now(), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if st, err := os.Stat(filepath.Join(d.Dir, d.filename("key"))); err != nil || st.Mode().Perm() != 0600 {
		t.Fatal(st, err)
	}
	outside := filepath.Join(dir, "outside")
	if err := os.WriteFile(outside, []byte(`private`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(d.Dir, d.filename("link"))); err != nil {
		t.Fatal(err)
	}
	if b, _, _, _ := d.Get(ctx, "link", true); b != nil {
		t.Fatal("followed symlink")
	}
	if err := d.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _, _, _ := other.Get(ctx, "key", true); b == nil {
		t.Fatal("cleared another namespace")
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "private" {
		t.Fatal(string(b), err)
	}
	if err := os.WriteFile(filepath.Join(d.Dir, d.filename("broken")), []byte(`not json`), 0600); err != nil {
		t.Fatal(err)
	}
	if b, _, _, _ := d.Get(ctx, "broken", true); b != nil {
		t.Fatal("corrupt hit")
	}
	if err := os.Chmod(d.Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := d.Get(ctx, "key", true); err == nil {
		t.Fatal("accepted public directory")
	}
}
func TestDiskConcurrentWritersAndBounds(t *testing.T) {
	ctx := context.Background()
	d := NewDisk(t.TempDir(), "work")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			other := NewDisk(filepath.Dir(d.Dir), "work")
			if err := other.Put(ctx, fmt.Sprint(i), "detail", []byte(`{}`), time.Now(), 30*time.Second); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	s, err := d.Status(ctx)
	if err != nil || s.Entries != 8 {
		t.Fatal(s, err)
	}
	// Seed near the limits directly, then assert that normal writes enforce both bounds.
	for i := 0; i < 1001; i++ {
		key := fmt.Sprintf("seed%d", i)
		e := diskEntry{1, d.Namespace, digest(key), "detail", time.Now().Add(-time.Hour), time.Minute, json.RawMessage(`{}`)}
		b, _ := json.Marshal(e)
		if err := os.WriteFile(filepath.Join(d.Dir, d.filename(key)), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Put(ctx, "last", "detail", []byte(`{}`), time.Now(), time.Minute); err != nil {
		t.Fatal(err)
	}
	s, err = d.Status(ctx)
	if err != nil || s.Entries > 1000 {
		t.Fatal(s, err)
	}
	for i := 0; i < 8; i++ {
		if err := d.Put(ctx, fmt.Sprintf("large%d", i), "search", []byte(`"`+strings.Repeat("a", 3*1024*1024)+`"`), time.Now(), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	s, err = d.Status(ctx)
	if err != nil || s.Bytes > maxDiskBytes {
		t.Fatal(s, err)
	}
}
