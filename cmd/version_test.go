package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()
	if info.Version == "" {
		t.Errorf("expected non-empty Version, got empty")
	}
	if info.GoVersion == "" {
		t.Errorf("expected non-empty GoVersion, got empty")
	}
	if info.Platform == "" {
		t.Errorf("expected non-empty Platform, got empty")
	}
}

func TestVersionCmd_JSON(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	oldBuildDate := BuildDate
	defer func() {
		Version = oldVersion
		Commit = oldCommit
		BuildDate = oldBuildDate
	}()

	Version = "v0.1.0-test"
	Commit = "abc1234"
	BuildDate = "2026-08-19T12:00:00Z"

	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs([]string{"version", "--json"})

	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info := GetBuildInfo()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal info: %v", err)
	}

	var parsed BuildInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if parsed.Version != "v0.1.0-test" {
		t.Errorf("expected Version 'v0.1.0-test', got '%s'", parsed.Version)
	}
	if parsed.Commit != "abc1234" {
		t.Errorf("expected Commit 'abc1234', got '%s'", parsed.Commit)
	}
}

func TestVersionCmd_Plain(t *testing.T) {
	oldVersion := Version
	defer func() { Version = oldVersion }()
	Version = "v0.1.0"

	info := GetBuildInfo()
	if !strings.Contains(info.Version, "v0.1.0") {
		t.Errorf("expected version to contain v0.1.0, got %s", info.Version)
	}
}

func TestVersionCmd_OutputFlag(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	oldBuildDate := BuildDate
	defer func() {
		Version = oldVersion
		Commit = oldCommit
		BuildDate = oldBuildDate
	}()

	Version = "v0.1.0-output-test"
	Commit = "fedcba9"
	BuildDate = "2026-08-19T14:00:00Z"

	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs([]string{"version", "-o", "json"})

	err := RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing 'robo version -o json': %v", err)
	}

	info := GetBuildInfo()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal info: %v", err)
	}

	var parsed BuildInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if parsed.Version != "v0.1.0-output-test" {
		t.Errorf("expected Version 'v0.1.0-output-test', got '%s'", parsed.Version)
	}
	if parsed.Commit != "fedcba9" {
		t.Errorf("expected Commit 'fedcba9', got '%s'", parsed.Commit)
	}
}

