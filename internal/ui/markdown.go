package ui

import (
	"charm.land/glamour/v2"
)

// RenderMarkdown renders markdown text using Glamour v2 with automatic dark/light theme support.
func RenderMarkdown(md string, width int) (string, error) {
	if width <= 0 {
		width = 80
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("auto"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Graceful fallback to raw markdown
		return md, nil
	}

	out, err := r.Render(md)
	if err != nil {
		return md, nil
	}

	return out, nil
}
