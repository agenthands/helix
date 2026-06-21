package forwarder

import (
	"context"
	"io"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
)

// GRPCClientStream abstracts the client side of the gRPC bidirectional stream
// for testability. It mirrors internal/mcp.GRPCStream (the server-side bridge
// uses the identical Recv/Send shape) so that the BidiStreamingClient returned
// by client.StreamMCP(ctx) satisfies it and tests can substitute a fake.
type GRPCClientStream interface {
	Recv() (*serenav1.MCPMessage, error)
	Send(*serenav1.MCPMessage) error
}

// GRPCClientTransport bridges the MCP SDK *client* onto a gRPC bidirectional
// stream. It is the strict CLIENT-side inversion of internal/mcp.GRPCTransport:
// same io.Pipe-pair shape and newline-delimited framing, opposite direction,
// and — unlike the server side — NO replay of a pre-consumed first message
// (the client never pre-consumes a stream message before building the transport).
//
// Data flow (inverse of the server bridge):
//
//	gRPC stream.Recv() -> clientWriter -> IOTransport.Reader -> MCP SDK client reads
//	MCP SDK client writes -> IOTransport.Writer -> serverReader -> gRPC stream.Send()
type GRPCClientTransport struct {
	stream    GRPCClientStream
	sessionID string
}

// NewGRPCClientTransport creates a client transport backed by a gRPC stream.
// sessionID is stamped onto every MCPMessage Sent toward the daemon so the
// daemon's session isolation keys this one-shot call correctly.
func NewGRPCClientTransport(stream GRPCClientStream, sessionID string) *GRPCClientTransport {
	return &GRPCClientTransport{
		stream:    stream,
		sessionID: sessionID,
	}
}

// Connect returns a Connection that bridges the gRPC stream to the MCP SDK
// client. It spawns two pump goroutines between gRPC and io.Pipe pairs.
//
// This is the inversion of internal/mcp.GRPCTransport.Connect: the SDK client
// reads what the stream Recv's, and the stream Sends what the SDK client writes.
// There is deliberately no first-message replay branch (the client side never
// pre-consumes a message off the stream).
func (t *GRPCClientTransport) Connect(ctx context.Context) (mcpsdk.Connection, error) {
	// Pipe feeding the SDK client reader: gRPC Recv -> clientWriter, IOTransport
	// reads from clientReader.
	clientReader, clientWriter := io.Pipe()
	// Pipe draining the SDK client writer: IOTransport writes to serverWriter,
	// the drain goroutine reads from serverReader and Sends to the stream.
	serverReader, serverWriter := io.Pipe()

	// Goroutine: gRPC stream.Recv() -> clientWriter (feeds SDK client reader).
	// On stream end/error, close the reader pipe WITH the error so the SDK
	// client terminates cleanly instead of hanging (RESEARCH Pitfall 3).
	go func() {
		defer clientWriter.Close()
		for {
			msg, err := t.stream.Recv()
			if err != nil {
				clientWriter.CloseWithError(err)
				return
			}
			// Newline-delimited JSON: the IOTransport reader expects each
			// message terminated by '\n'.
			if _, err := clientWriter.Write(msg.Payload); err != nil {
				return
			}
			if _, err := clientWriter.Write([]byte("\n")); err != nil {
				return
			}
		}
	}()

	// Goroutine: serverReader -> gRPC stream.Send() (drains SDK client output).
	// The SDK client writes newline-delimited JSON; split on '\n' and Send each
	// non-empty line as its own MCPMessage (mirrors the server-side drain loop).
	go func() {
		defer serverReader.Close()
		buf := make([]byte, 0, 64*1024)
		tmp := make([]byte, 4096)
		for {
			n, err := serverReader.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				for {
					nlIdx := -1
					for i, b := range buf {
						if b == '\n' {
							nlIdx = i
							break
						}
					}
					if nlIdx < 0 {
						break
					}
					line := buf[:nlIdx]
					buf = buf[nlIdx+1:]
					if len(line) > 0 {
						sendErr := t.stream.Send(&serenav1.MCPMessage{
							Payload:   append([]byte(nil), line...),
							SessionId: t.sessionID,
						})
						if sendErr != nil {
							serverReader.CloseWithError(sendErr)
							return
						}
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	ioTransport := &mcpsdk.IOTransport{
		Reader: clientReader,
		Writer: serverWriter,
	}
	return ioTransport.Connect(ctx)
}
