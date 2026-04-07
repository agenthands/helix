package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads a single Content-Length framed JSON-RPC message from r.
// It parses the Content-Length header, ignores other headers (e.g., Content-Type),
// and reads the exact number of body bytes specified.
func ReadMessage(r *bufio.Reader) (json.RawMessage, error) {
	var contentLen int
	var foundContentLength bool

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line separates headers from body.
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLen, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			foundContentLength = true
		}
		// Ignore other headers like Content-Type.
	}

	if !foundContentLength {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	if contentLen <= 0 {
		return nil, fmt.Errorf("invalid Content-Length: %d", contentLen)
	}

	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("reading body (%d bytes): %w", contentLen, err)
	}

	return json.RawMessage(body), nil
}

// WriteMessage writes a Content-Length framed JSON-RPC message to w.
// It writes "Content-Length: N\r\n\r\n" followed by the message body.
func WriteMessage(w io.Writer, msg json.RawMessage) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg))
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("writing body: %w", err)
	}
	return nil
}
