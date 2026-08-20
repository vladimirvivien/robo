package ui_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/ui"
)

func TestFormatters(t *testing.T) {
	data := ui.OutputData{
		Response:    "\x1b[32mHere is the command:\x1b[0m\n```powershell\nGet-Date\n```",
		Explanation: "\x1b[32mHere is the command:\x1b[0m",
		Command:     "Get-Date",
		Provider:    "litertlm",
		Model:       "gemma-4-E2B-it",
		Local:       true,
	}

	t.Run("JSONFormatter_NoANSI", func(t *testing.T) {
		f, err := ui.NewFormatter("json", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}

		raw := buf.String()
		if strings.Contains(raw, "\x1b") {
			t.Errorf("JSON output contains ANSI escape sequences: %s", raw)
		}

		var parsed ui.OutputData
		if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid json output: %v, raw: %s", err, buf.String())
		}
		if parsed.Command != "Get-Date" {
			t.Errorf("expected command Get-Date, got %s", parsed.Command)
		}
		if parsed.Provider != "litertlm" {
			t.Errorf("expected provider litertlm, got %s", parsed.Provider)
		}
		if !parsed.Local {
			t.Errorf("expected Local to be true")
		}
	})

	t.Run("PlainFormatter_CleanOutput", func(t *testing.T) {
		f, err := ui.NewFormatter("plain", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "\x1b") {
			t.Errorf("plain output contains ANSI escape sequences: %s", out)
		}
		if !strings.Contains(out, "Get-Date") || !strings.Contains(out, "Here is the command:") {
			t.Errorf("unexpected plain output: %s", out)
		}
	})

	t.Run("CodeFormatter_CleanCommandOnly", func(t *testing.T) {
		f, err := ui.NewFormatter("code", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}
		out := strings.TrimSpace(buf.String())
		if strings.Contains(out, "\x1b") {
			t.Errorf("code output contains ANSI escape sequences: %s", out)
		}
		if out != "Get-Date" {
			t.Errorf("expected code Get-Date, got %q", out)
		}
	})

	t.Run("MarkdownFormatter", func(t *testing.T) {
		f, err := ui.NewFormatter("markdown", true, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Here is the command:") {
			t.Errorf("expected explanation in card, got %s", out)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := ui.NewFormatter("unsupported-format-xyz", false, 80)
		if err == nil {
			t.Errorf("expected error for unsupported format, got nil")
		}
	})
}
