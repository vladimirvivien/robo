package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// OutputData encapsulates all metadata and content generated for a request.
type OutputData struct {
	Response    string `json:"response"`
	Explanation string `json:"explanation,omitempty"`
	Command     string `json:"command,omitempty"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Local       bool   `json:"local"`
	TokensUsed  int    `json:"tokens_used,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Formatter defines the contract for rendering response output across different presentation modes.
type Formatter interface {
	Format(w io.Writer, data OutputData) error
}

// MarkdownFormatter renders rich styled cards with Lipgloss and Glamour in interactive terminals.
type MarkdownFormatter struct {
	Interactive bool
	Width       int
}

// Format renders output in Markdown or Lipgloss card framing.
func (f *MarkdownFormatter) Format(w io.Writer, data OutputData) error {
	if !f.Interactive {
		_, err := fmt.Fprintln(w, data.Response)
		return err
	}

	cardWidth := CappedWidth(f.Width)
	renderWidth := max(cardWidth-6, 30)

	if data.Command == "" {
		rendered, _ := RenderMarkdown(data.Response, renderWidth)
		card := CardWithWidth("", strings.TrimSpace(rendered), "", cardWidth)
		_, err := fmt.Fprintln(w, card)
		return err
	}

	// If command is present, render explanation in card if non-empty
	if data.Explanation != "" {
		rendered, _ := RenderMarkdown(data.Explanation, renderWidth)
		card := CardWithWidth("", strings.TrimSpace(rendered), "", cardWidth)
		if _, err := fmt.Fprintln(w, card); err != nil {
			return err
		}
	}

	return nil
}

// PlainFormatter renders clean, unstyled plain text without ANSI escapes.
type PlainFormatter struct{}

// Format renders output in unstyled plain text.
func (f *PlainFormatter) Format(w io.Writer, data OutputData) error {
	if data.Explanation != "" && data.Command != "" {
		if _, err := fmt.Fprintln(w, data.Explanation); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, data.Command)
		return err
	}
	_, err := fmt.Fprintln(w, data.Response)
	return err
}

// JSONFormatter serializes the response and metadata to indented JSON.
type JSONFormatter struct {
	Pretty bool
}

// Format serializes output data as structured JSON.
func (f *JSONFormatter) Format(w io.Writer, data OutputData) error {
	var encoded []byte
	var err error
	if f.Pretty {
		encoded, err = json.MarshalIndent(data, "", "  ")
	} else {
		encoded, err = json.Marshal(data)
	}
	if err != nil {
		return fmt.Errorf("format json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// CodeFormatter outputs only the extracted shell command.
type CodeFormatter struct{}

// Format renders only the command portion of output data.
func (f *CodeFormatter) Format(w io.Writer, data OutputData) error {
	if data.Command != "" {
		_, err := fmt.Fprintln(w, data.Command)
		return err
	}
	_, err := fmt.Fprintln(w, data.Response)
	return err
}

// NewFormatter resolves the appropriate Formatter implementation for the given mode name.
func NewFormatter(mode string, interactive bool, termWidth int) (Formatter, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "markdown", "md", "":
		return &MarkdownFormatter{
			Interactive: interactive,
			Width:       termWidth,
		}, nil
	case "json":
		return &JSONFormatter{Pretty: true}, nil
	case "plain", "text", "raw":
		return &PlainFormatter{}, nil
	case "code", "cmd", "command":
		return &CodeFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q (expected markdown, plain, json, code)", mode)
	}
}
