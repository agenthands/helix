package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// NotificationFunc is a callback for handling incoming notifications.
type NotificationFunc func(method string, params json.RawMessage)

// Conn is a bidirectional JSON-RPC 2.0 connection over a Content-Length framed stream.
// It supports Call (request-response), Notify (fire-and-forget), and Listen (dispatch loop).
type Conn struct {
	rwc    io.ReadWriteCloser
	reader *bufio.Reader

	// Session prefix for ID generation to avoid collisions (per research Pitfall 4).
	sessionPrefix string

	// Atomic counter for generating unique request IDs.
	nextID atomic.Int64

	// mu protects the writer and pending map.
	mu      sync.Mutex
	pending map[string]chan *Response
	closed  bool

	// OnNotification is called for incoming notifications during Listen.
	OnNotification NotificationFunc
}

// NewConn creates a new JSON-RPC connection wrapping the given stream.
// sessionPrefix is prepended to request IDs to avoid collisions when multiple
// sessions share an LS worker (per Pitfall 4: sequential integer IDs).
func NewConn(rwc io.ReadWriteCloser, sessionPrefix string) *Conn {
	return &Conn{
		rwc:           rwc,
		reader:        bufio.NewReader(rwc),
		sessionPrefix: sessionPrefix,
		pending:       make(map[string]chan *Response),
	}
}

// nextRequestID generates a unique session-prefixed request ID.
func (c *Conn) nextRequestID() string {
	n := c.nextID.Add(1)
	return fmt.Sprintf("%s:%d", c.sessionPrefix, n)
}

// Send sends a JSON-RPC request and returns the assigned ID.
// It does not wait for a response; use Call for request-response.
func (c *Conn) Send(ctx context.Context, method string, params interface{}) (string, error) {
	id := c.nextRequestID()
	req, err := NewRequest(id, method, params)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return "", fmt.Errorf("connection closed")
	}
	err = WriteMessage(c.rwc, data)
	c.mu.Unlock()

	if err != nil {
		return "", fmt.Errorf("writing request: %w", err)
	}
	return id, nil
}

// Call sends a JSON-RPC request and waits for the response.
// The result is unmarshaled into the provided result pointer.
func (c *Conn) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.nextRequestID()
	req, err := NewRequest(id, method, params)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	// Register pending response channel before sending.
	ch := make(chan *Response, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("connection closed")
	}
	c.pending[id] = ch
	err = WriteMessage(c.rwc, data)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("writing request: %w", err)
	}

	// Wait for response or context cancellation.
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshaling result: %w", err)
			}
		}
		return nil
	}
}

// Notify sends a JSON-RPC notification (no ID, no response expected).
func (c *Conn) Notify(ctx context.Context, method string, params interface{}) error {
	notif, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("creating notification: %w", err)
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshaling notification: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	return WriteMessage(c.rwc, data)
}

// Listen reads messages from the connection and dispatches them.
// Responses are routed to pending Call waiters. Notifications are delivered
// to the OnNotification callback. Listen blocks until the context is
// cancelled or the connection is closed.
func (c *Conn) Listen(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := ReadMessage(c.reader)
		if err != nil {
			c.mu.Lock()
			c.closed = true
			// Wake up all pending callers.
			for id, ch := range c.pending {
				ch <- &Response{
					Error: &ResponseError{
						Code:    -32000,
						Message: fmt.Sprintf("connection closed: %v", err),
					},
				}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return fmt.Errorf("reading message: %w", err)
		}

		// Determine message type by peeking at the JSON.
		var peek struct {
			ID     interface{}     `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *ResponseError  `json:"error"`
		}
		if err := json.Unmarshal(msg, &peek); err != nil {
			continue // Skip malformed messages.
		}

		if peek.Method != "" && peek.ID == nil {
			// Notification (has method, no ID).
			if c.OnNotification != nil {
				var notif Notification
				if err := json.Unmarshal(msg, &notif); err == nil {
					c.OnNotification(notif.Method, notif.Params)
				}
			}
			continue
		}

		if peek.ID != nil && peek.Method == "" {
			// Response (has ID, no method).
			var resp Response
			if err := json.Unmarshal(msg, &resp); err != nil {
				continue
			}
			// Convert ID to string key for lookup.
			idStr := fmt.Sprintf("%v", resp.ID)
			c.mu.Lock()
			ch, ok := c.pending[idStr]
			if ok {
				delete(c.pending, idStr)
			}
			c.mu.Unlock()
			if ok {
				ch <- &resp
			}
			continue
		}

		// Request from server (has both ID and method) -- ignore for now.
		// Server-to-client requests can be handled by extending this.
	}
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	c.closed = true
	for id, ch := range c.pending {
		ch <- &Response{
			Error: &ResponseError{
				Code:    -32000,
				Message: "connection closed",
			},
		}
		delete(c.pending, id)
	}
	c.mu.Unlock()
	return c.rwc.Close()
}
