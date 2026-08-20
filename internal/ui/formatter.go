package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// OutputData encapsulates all metadata and content generated for a request.
type OutputData struct {
	Response    string `json:"response"`
	Explanation string `json:"explanation,omitempty"`
	Command     string `json:"command,omitempty"`
	Output      string `json:"output,omitempty"`
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
		trimmed := strings.TrimSpace(data.Response)
		if trimmed == "" || trimmed == "Command executed successfully." {
			return nil
		}
		rendered, _ := RenderMarkdown(trimmed, renderWidth)
		renderedTrimmed := strings.TrimSpace(rendered)
		if renderedTrimmed == "" {
			return nil
		}
		card := CardWithWidth("", renderedTrimmed, "", cardWidth)
		_, err := fmt.Fprintln(w, card)
		return err
	}

	// If command is present, render explanation in card if non-empty
	if strings.TrimSpace(data.Explanation) != "" {
		rendered, _ := RenderMarkdown(strings.TrimSpace(data.Explanation), renderWidth)
		renderedTrimmed := strings.TrimSpace(rendered)
		if renderedTrimmed != "" {
			card := CardWithWidth("", renderedTrimmed, "", cardWidth)
			if _, err := fmt.Fprintln(w, card); err != nil {
				return err
			}
		}
	}

	return nil
}

// PlainFormatter renders clean, unstyled plain text without ANSI escapes.
type PlainFormatter struct{}

// Format renders output in unstyled plain text without UI relics or ANSI codes.
func (f *PlainFormatter) Format(w io.Writer, data OutputData) error {
	out := ansi.Strip(strings.TrimSpace(data.Output))
	if out != "" {
		_, err := fmt.Fprintln(w, out)
		return err
	}

	expl := ansi.Strip(strings.TrimSpace(data.Explanation))
	cmd := ansi.Strip(strings.TrimSpace(data.Command))

	if expl != "" && cmd != "" {
		if _, err := fmt.Fprintln(w, expl); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, cmd)
		return err
	}
	if cmd != "" {
		_, err := fmt.Fprintln(w, cmd)
		return err
	}

	raw := ansi.Strip(strings.TrimSpace(data.Response))
	if raw != "" {
		_, err := fmt.Fprintln(w, raw)
		return err
	}
	return nil
}

// JSONFormatter serializes the response and metadata to indented JSON.
type JSONFormatter struct {
	Pretty bool
}

// Format serializes output data as structured JSON without ANSI codes.
func (f *JSONFormatter) Format(w io.Writer, data OutputData) error {
	cleanData := OutputData{
		Response:    ansi.Strip(strings.TrimSpace(data.Response)),
		Explanation: ansi.Strip(strings.TrimSpace(data.Explanation)),
		Command:     ansi.Strip(strings.TrimSpace(data.Command)),
		Output:      ansi.Strip(strings.TrimSpace(data.Output)),
		Provider:    data.Provider,
		Model:       data.Model,
		Local:       data.Local,
		TokensUsed:  data.TokensUsed,
		Error:       ansi.Strip(strings.TrimSpace(data.Error)),
	}

	var encoded []byte
	var err error
	if f.Pretty {
		encoded, err = json.MarshalIndent(cleanData, "", "  ")
	} else {
		encoded, err = json.Marshal(cleanData)
	}
	if err != nil {
		return fmt.Errorf("format json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// CodeFormatter outputs only the extracted shell command without fences or ANSI relics.
type CodeFormatter struct{}

// Format renders only the command portion of output data.
func (f *CodeFormatter) Format(w io.Writer, data OutputData) error {
	cmd := ansi.Strip(strings.TrimSpace(data.Command))
	if cmd != "" {
		_, err := fmt.Fprintln(w, cmd)
		return err
	}

	raw := ansi.Strip(strings.TrimSpace(data.Response))
	if raw != "" {
		_, err := fmt.Fprintln(w, raw)
		return err
	}
	return nil
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
