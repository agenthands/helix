// Package jsonrpc implements a thin JSON-RPC 2.0 codec for communicating
// with Language Server child processes over Content-Length framed stdio.
package jsonrpc

import "encoding/json"

const jsonrpcVersion = "2.0"

// Request is a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError is a JSON-RPC 2.0 error object.
type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *ResponseError) Error() string {
	return e.Message
}

// Notification is a JSON-RPC 2.0 notification (no ID, no response expected).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewRequest creates a new JSON-RPC 2.0 request.
func NewRequest(id interface{}, method string, params interface{}) (*Request, error) {
	var raw json.RawMessage
	if params != nil {
		var err error
		raw, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}
	return &Request{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Method:  method,
		Params:  raw,
	}, nil
}

// NewNotification creates a new JSON-RPC 2.0 notification.
func NewNotification(method string, params interface{}) (*Notification, error) {
	var raw json.RawMessage
	if params != nil {
		var err error
		raw, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}
	return &Notification{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  raw,
	}, nil
}
