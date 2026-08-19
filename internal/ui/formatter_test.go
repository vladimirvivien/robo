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
		Response:    "Here is the command:\n```powershell\nGet-Date\n```",
		Explanation: "Here is the command:",
		Command:     "Get-Date",
		Provider:    "litertlm",
		Model:       "gemma-4-E2B-it",
		Local:       true,
	}

	t.Run("JSONFormatter", func(t *testing.T) {
		f, err := ui.NewFormatter("json", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
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

	t.Run("PlainFormatter", func(t *testing.T) {
		f, err := ui.NewFormatter("plain", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Get-Date") || !strings.Contains(out, "Here is the command:") {
			t.Errorf("unexpected plain output: %s", out)
		}
	})

	t.Run("CodeFormatter", func(t *testing.T) {
		f, err := ui.NewFormatter("code", false, 80)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("format error: %v", err)
		}
		out := strings.TrimSpace(buf.String())
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
