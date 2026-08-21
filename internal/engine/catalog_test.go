package engine_test

import (
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/engine"
)

func TestModelCatalog(t *testing.T) {
	if len(engine.ModelCatalog) == 0 {
		t.Fatal("expected ModelCatalog to contain models")
	}

	for _, m := range engine.ModelCatalog {
		if m.ID == "" {
			t.Errorf("model missing ID: %+v", m)
		}
		if m.URL == "" || !strings.HasPrefix(m.URL, "https://") {
			t.Errorf("model %s has invalid URL: %s", m.ID, m.URL)
		}
		if m.Filename == "" || !strings.HasSuffix(m.Filename, ".litertlm") {
			t.Errorf("model %s has invalid Filename: %s", m.ID, m.Filename)
		}
	}
}

func TestLookupModel(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantFound bool
	}{
		{"litert-community/gemma-4-E4B-it", "litert-community/gemma-4-E4B-it", true},
		{"litert-community/gemma-4-E2B-it", "litert-community/gemma-4-E2B-it", true},
		{"litert-community/gemma-4-12B-it", "litert-community/gemma-4-12B-it", true},
		{"litert-community/qwen3-4b-it", "litert-community/qwen3-4b-it", true},
		{"gemma-4-4b", "litert-community/gemma-4-E4B-it", true},
		{"gemma-4-e2b", "litert-community/gemma-4-E2B-it", true},
		{"gemma-4-12b", "litert-community/gemma-4-12B-it", true},
		{"qwen3-4b", "litert-community/qwen3-4b-it", true},
		{"nonexistent-model-xyz", "", false},
	}

	for _, tc := range tests {
		info, found := engine.LookupModel(tc.input)
		if found != tc.wantFound {
			t.Errorf("LookupModel(%q) found=%v, want %v", tc.input, found, tc.wantFound)
		}
		if found && info.ID != tc.wantID {
			t.Errorf("LookupModel(%q) ID=%q, want %q", tc.input, info.ID, tc.wantID)
		}
	}
}

func TestResolveModelTarget(t *testing.T) {
	t.Run("direct URL", func(t *testing.T) {
		url, filename := engine.ResolveModelTarget("https://example.com/custom/model.litertlm")
		if url != "https://example.com/custom/model.litertlm" {
			t.Errorf("unexpected URL: %s", url)
		}
		if filename != "model.litertlm" {
			t.Errorf("unexpected filename: %s", filename)
		}
	})

	t.Run("catalog lookup", func(t *testing.T) {
		url, filename := engine.ResolveModelTarget("gemma-4-e2b")
		if !strings.Contains(url, "gemma-4-E2B-it.litertlm") {
			t.Errorf("unexpected resolved URL: %s", url)
		}
		if filename != "gemma-4-E2B-it.litertlm" {
			t.Errorf("unexpected filename: %s", filename)
		}
	})
}
