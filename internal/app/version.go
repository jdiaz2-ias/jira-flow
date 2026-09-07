// Package app contains application use cases independent of CLI and TUI frameworks.
package app

import "runtime"

type VersionInfo struct {
	Version   string
	Commit    string
	GoVersion string
	OS        string
	Arch      string
}

func BuildInfo(version, commit string) VersionInfo {
	return VersionInfo{Version: version, Commit: commit, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
}
