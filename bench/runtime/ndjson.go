package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ndjsonDeadline bounds how long a wait() blocks for a single response before
// recording a timeout. It mirrors the per-call read deadline of the legacy
// daemon-forwarder driver (and the integration test's 10s read deadline).
const ndjsonDeadline = 10 * time.Second

// This file owns the NDJSON JSON-RPC reader/dispatcher used to drive a process
// that IS an MCP endpoint over its own stdio — specifically the standalone
// cmd/helix-bench-rag server (rag.go:driveRAGServer). That server is the MCP
// endpoint by construction (StdioTransport, no forwarder), so hand-framed NDJSON
// over its stdin/stdout is the correct transport.
//
// NOTE (Phase 94 fix(94-02)): these helpers are NOT a daemon-driving path. The
// daemon-facing bench driver (drive.go:driveScript) was migrated off the deleted
// `helix --mode=stdio` forwarder head onto the retained gRPC StreamMCP wire
// (forwarder.OpenSession) and no longer uses these helpers. They remain ONLY for
// the bench-rag standalone server's own stdio, which is unrelated to the retired
// agent-facing stdio MCP head.

// jsonrpcResp is a minimally-parsed JSON-RPC response line.
type jsonrpcResp struct {
	id     int
	raw    json.RawMessage // the full response line (for StepResult.Response)
	rpcErr string          // non-empty when the response carried a JSON-RPC error
}

// readResponses scans a reader for NDJSON JSON-RPC responses and publishes each
// one (keyed by id) onto respCh. It returns when the reader reaches EOF or
// errors, or when done is closed.
//
// WR-02: the send is non-blocking against done — once the drive loop returns and
// closes done, no one drains respCh, so a blocking send on a full buffer would
// park this goroutine forever. Selecting on done lets the reader exit instead of
// leaking.
func readResponses(stdout io.Reader, respCh chan<- jsonrpcResp, done <-chan struct{}) {
	reader := bufio.NewReaderSize(stdout, 256*1024)
	for {
		line, err := reader.ReadString('\n')
		if s := strings.TrimSpace(line); s != "" {
			if parsed, ok := parseRPCLine([]byte(s)); ok {
				select {
				case respCh <- parsed:
				case <-done:
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// parseRPCLine extracts the id and any error message from one JSON-RPC response
// line. Lines without a numeric id (e.g. notifications) are ignored (ok=false).
func parseRPCLine(line []byte) (jsonrpcResp, bool) {
	var env struct {
		ID    *json.Number `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &env); err != nil || env.ID == nil {
		return jsonrpcResp{}, false
	}
	idInt, err := env.ID.Int64()
	if err != nil {
		return jsonrpcResp{}, false
	}
	r := jsonrpcResp{id: int(idInt), raw: json.RawMessage(append([]byte(nil), line...))}
	switch {
	case env.Error != nil:
		r.rpcErr = env.Error.Message
		if r.rpcErr == "" {
			r.rpcErr = "jsonrpc error"
		}
	case env.Result != nil && env.Result.IsError:
		if len(env.Result.Content) > 0 && env.Result.Content[0].Text != "" {
			r.rpcErr = env.Result.Content[0].Text
		} else {
			r.rpcErr = "tool returned isError"
		}
	}
	return r, true
}

// respDispatcher demultiplexes a single response stream into id-keyed waits. It
// holds a pending buffer so a response for a not-yet-awaited id is RETAINED
// across wait() calls instead of being discarded (WR-03). It is single-consumer:
// all wait() calls happen on the drive goroutine, so pending needs no lock.
type respDispatcher struct {
	ch      <-chan jsonrpcResp
	pending map[int]jsonrpcResp
}

// wait returns the response with the given id, consulting the pending buffer
// first and otherwise reading from the channel — stashing any non-matching id
// into pending rather than dropping it — until the id arrives, the per-call
// deadline elapses, or the parent context is done.
func (d *respDispatcher) wait(ctx context.Context, id int) (jsonrpcResp, error) {
	if r, ok := d.pending[id]; ok {
		delete(d.pending, id)
		return r, nil
	}
	deadline := time.NewTimer(ndjsonDeadline)
	defer deadline.Stop()
	for {
		select {
		case r, ok := <-d.ch:
			if !ok {
				return jsonrpcResp{}, fmt.Errorf("stdout closed before response id=%d", id)
			}
			if r.id == id {
				return r, nil
			}
			d.pending[r.id] = r
		case <-deadline.C:
			return jsonrpcResp{}, fmt.Errorf("no response for id=%d within %s", id, ndjsonDeadline)
		case <-ctx.Done():
			return jsonrpcResp{}, fmt.Errorf("context done waiting for response id=%d: %w", id, ctx.Err())
		}
	}
}
