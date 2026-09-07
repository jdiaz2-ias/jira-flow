package actionlog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/domain"
)

func TestPrivateMinimalHistoryAndRetention(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "actions")
	os.MkdirAll(dir, 0700)
	expired := filepath.Join(dir, strings.Repeat("a", 32)+".json")
	os.WriteFile(expired, []byte(`{}`), 0600)
	old := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(expired, old, old)
	unrelated := filepath.Join(dir, "notes.txt")
	os.WriteFile(unrelated, []byte("keep"), 0600)
	id, e := Save(context.Background(), dir, Record{Key: "APP-1", Intent: domain.IntentDone, TransitionID: "31", Result: domain.ApplyVerified})
	if e != nil || len(id) != 32 {
		t.Fatal(id, e)
	}
	b, e := os.ReadFile(filepath.Join(dir, id+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]any
	if json.Unmarshal(b, &fields) != nil || len(fields) != 6 || fields["result"] != "verified" {
		t.Fatal(string(b))
	}
	info, _ := os.Stat(filepath.Join(dir, id+".json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if _, e = os.Stat(expired); !os.IsNotExist(e) {
		t.Fatal("expired entry retained")
	}
	if _, e = os.Stat(unrelated); e != nil {
		t.Fatal("unrelated file removed")
	}
}
