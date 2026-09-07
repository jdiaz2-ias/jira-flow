// Package actionlog stores minimal action metadata, never fields or credentials.
package actionlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/domain"
)

type Record struct {
	ID           string            `json:"id"`
	Key          string            `json:"key"`
	Intent       domain.Intent     `json:"intent"`
	TransitionID string            `json:"transition_id,omitempty"`
	At           time.Time         `json:"at"`
	Result       domain.ApplyState `json:"result"`
}

func Save(ctx context.Context, directory string, record Record) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	record.ID = hex.EncodeToString(id)
	record.At = time.Now().UTC()
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(directory, ".action-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err = os.Rename(f.Name(), filepath.Join(directory, record.ID+".json")); err != nil {
		return "", err
	}
	// Each record has a unique filename, so concurrent writers never replace data.
	// Prune only well-formed jflow record names older than the retention period.
	entries, err := os.ReadDir(directory)
	if err != nil {
		return record.ID, err
	}
	cutoff := record.At.Add(-7 * 24 * time.Hour)
	for _, entry := range entries {
		stem := strings.TrimSuffix(entry.Name(), ".json")
		if len(stem) != 32 || !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		if _, e := hex.DecodeString(stem); e != nil {
			continue
		}
		info, e := entry.Info()
		if e == nil && info.Mode().IsRegular() && info.ModTime().Before(cutoff) {
			if e = os.Remove(filepath.Join(directory, entry.Name())); e != nil {
				return record.ID, e
			}
		}
	}
	return record.ID, nil
}
