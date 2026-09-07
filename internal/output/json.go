// Package output owns the public wire format, independently of domain structs.
package output

import (
	"encoding/json"
	"errors"
	"io"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}
type Envelope struct {
	SchemaVersion int            `json:"schema_version"`
	OK            bool           `json:"ok"`
	Data          any            `json:"data"`
	Meta          map[string]any `json:"meta"`
	Warnings      []string       `json:"warnings"`
	Error         *Error         `json:"error"`
}

type Version struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

func VersionData(info app.VersionInfo) Version {
	return Version{info.Version, info.Commit, info.GoVersion, info.OS, info.Arch}
}

func Success(data any) Envelope {
	return Envelope{SchemaVersion: 1, OK: true, Data: data, Meta: map[string]any{}, Warnings: []string{}}
}

func Failure(err error) Envelope {
	e := &Error{Code: string(domain.Internal), Message: "Internal error.", Details: map[string]any{}}
	var public *domain.Error
	if errors.As(err, &public) {
		e.Code, e.Message, e.Retryable = string(public.Kind), public.Message, public.Retryable
	}
	return Envelope{SchemaVersion: 1, OK: false, Meta: map[string]any{}, Warnings: []string{}, Error: e}
}

// Marshal first so serialization errors never leave a partial JSON document.
func Write(w io.Writer, envelope Envelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
