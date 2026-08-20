package engine

import (
	"path/filepath"
	"strings"
)

// ModelInfo describes a supported model in the pre-resolved static catalog.
type ModelInfo struct {
	ID          string // Namespaced ID, e.g. "litert-community/gemma-4-E4B-it"
	Name        string // Display name
	Description string // UI prompt description
	URL         string // Pre-resolved Hugging Face resolve URL
	Filename    string // Canonical filename on disk
	Size        string // Approximate download size
	Default     bool   // True if default selection
}

// ModelCatalog is the static registry of supported on-device models for robo.
var ModelCatalog = []ModelInfo{
	{
		ID:          "litert-community/gemma-4-E4B-it",
		Name:        "Gemma 4 4B",
		Description: "Gemma 4 4B  [~2.6 GB] • Recommended (Balanced reasoning & speed)",
		URL:         "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm",
		Filename:    "gemma-4-E4B-it.litertlm",
		Size:        "2.6 GB",
		Default:     true,
	},
	{
		ID:          "litert-community/gemma-4-E2B-it",
		Name:        "Gemma 4 2B",
		Description: "Gemma 4 2B  [~1.6 GB] • Fast & lightweight on-device",
		URL:         "https://huggingface.co/litert-community/gemma-4-E2B-it-litert-lm/resolve/main/gemma-4-E2B-it.litertlm",
		Filename:    "gemma-4-E2B-it.litertlm",
		Size:        "1.6 GB",
	},
	{
		ID:          "litert-community/gemma-4-12B-it",
		Name:        "Gemma 4 12B",
		Description: "Gemma 4 12B [~7.5 GB] • High capability reasoning & coding",
		URL:         "https://huggingface.co/litert-community/gemma-4-12B-it-litert-lm/resolve/main/gemma-4-12B-it.litertlm",
		Filename:    "gemma-4-12B-it.litertlm",
		Size:        "7.5 GB",
	},
	{
		ID:          "litert-community/gemma3-1b-it-int4",
		Name:        "Gemma 3 1B (int4)",
		Description: "Gemma 3 1B  [~580 MB] • Ultra-compact on-device SLM",
		URL:         "https://huggingface.co/litert-community/gemma3-1b-it-int4/resolve/main/gemma3-1b-it-int4.litertlm",
		Filename:    "gemma3-1b-it-int4.litertlm",
		Size:        "580 MB",
	},
	{
		ID:          "litert-community/qwen3-4b-it",
		Name:        "Qwen 3 4B (int8)",
		Description: "Qwen 3 4B   [~2.5 GB] • Multilingual & strong tool calling",
		URL:         "https://huggingface.co/litert-community/qwen3_4b_channelwise_int8_float32kv/resolve/main/qwen3_4b_channelwise_int8_float32kv.litertlm",
		Filename:    "qwen3_4b_channelwise_int8_float32kv.litertlm",
		Size:        "2.5 GB",
	},
}

// catalogLookup maps various aliases and filenames to the canonical ModelInfo.
var catalogLookup map[string]ModelInfo

func init() {
	catalogLookup = make(map[string]ModelInfo)
	for _, m := range ModelCatalog {
		catalogLookup[strings.ToLower(m.ID)] = m
		catalogLookup[strings.ToLower(m.Filename)] = m
		catalogLookup[strings.ToLower(strings.TrimSuffix(m.Filename, ".litertlm"))] = m

		// Common shorthand aliases
		switch m.ID {
		case "litert-community/gemma-4-E4B-it":
			catalogLookup["gemma-4-e4b"] = m
			catalogLookup["gemma-4-4b"] = m
			catalogLookup["4b"] = m
			catalogLookup["litert-community/gemma-4-e4b-it-litert-lm"] = m
		case "litert-community/gemma-4-E2B-it":
			catalogLookup["gemma-4-e2b"] = m
			catalogLookup["gemma-4-2b"] = m
			catalogLookup["2b"] = m
			catalogLookup["litert-community/gemma-4-e2b-it-litert-lm"] = m
		case "litert-community/gemma-4-12B-it":
			catalogLookup["gemma-4-12b"] = m
			catalogLookup["12b"] = m
			catalogLookup["litert-community/gemma-4-12b-it-litert-lm"] = m
		case "litert-community/gemma3-1b-it-int4":
			catalogLookup["gemma3-1b"] = m
			catalogLookup["gemma3-1b-it"] = m
			catalogLookup["1b"] = m
		case "litert-community/qwen3-4b-it":
			catalogLookup["qwen3-4b"] = m
			catalogLookup["qwen-4b"] = m
			catalogLookup["litert-community/qwen3_4b_channelwise_int8_float32kv"] = m
		}
	}
}

// LookupModel returns ModelInfo for a model identifier if registered in the static catalog.
func LookupModel(identifier string) (ModelInfo, bool) {
	clean := strings.ToLower(strings.TrimSpace(identifier))
	info, ok := catalogLookup[clean]
	return info, ok
}

// ResolveModelTarget maps an identifier, file path, or URL to the download URL and local filename.
func ResolveModelTarget(identifier string) (url string, filename string) {
	clean := strings.TrimSpace(identifier)
	if clean == "" {
		return "", ""
	}

	// 1. Direct HTTP/HTTPS URL
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
		return clean, filepath.Base(clean)
	}

	// 2. Direct file on disk
	if fileExists(clean) {
		return clean, filepath.Base(clean)
	}

	// 3. Pre-resolved static catalog lookup
	if info, ok := LookupModel(clean); ok {
		return info.URL, info.Filename
	}

	// 4. Passthrough
	filename = filepath.Base(clean)
	if !strings.HasSuffix(strings.ToLower(filename), ".litertlm") {
		filename += ".litertlm"
	}
	return clean, filename
}
