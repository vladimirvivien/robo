package cloud

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// SSEEvent represents a single Server-Sent Event payload.
type SSEEvent struct {
	Type string
	Data []byte
}

// ReadSSE reads and parses Server-Sent Events from an io.ReadCloser,
// calling the handle callback for each 'data:' event until EOF.
func ReadSSE(r io.Reader, handle func(event SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	// Allow larger SSE chunks (up to 1MB)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var currentType string
	var currentData bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line marks end of event
		if len(strings.TrimSpace(line)) == 0 {
			if currentData.Len() > 0 {
				data := bytes.TrimSpace(currentData.Bytes())
				currentData.Reset()
				if bytes.Equal(data, []byte("[DONE]")) {
					return nil
				}
				if err := handle(SSEEvent{Type: currentType, Data: data}); err != nil {
					return err
				}
			}
			currentType = ""
			continue
		}

		// Comment line, ignore
		if strings.HasPrefix(line, ":") {
			continue
		}

		if after, ok := strings.CutPrefix(line, "event:"); ok {
			currentType = strings.TrimSpace(after)
			continue
		}

		if after, ok := strings.CutPrefix(line, "data:"); ok {
			dataContent := after
			if currentData.Len() > 0 {
				currentData.WriteByte('\n')
			}
			currentData.WriteString(strings.TrimSpace(dataContent))
			continue
		}
	}

	// Process any remaining buffered event at EOF
	if currentData.Len() > 0 {
		data := bytes.TrimSpace(currentData.Bytes())
		if !bytes.Equal(data, []byte("[DONE]")) {
			return handle(SSEEvent{Type: currentType, Data: data})
		}
	}

	return scanner.Err()
}
