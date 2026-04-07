package mcp

import (
	"context"
	"io"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
)

// GRPCStream abstracts the gRPC bidirectional stream for testability.
type GRPCStream interface {
	Recv() (*serenav1.MCPMessage, error)
	Send(*serenav1.MCPMessage) error
}

// GRPCTransport bridges a gRPC bidirectional stream to the MCP SDK's Transport interface.
// It uses io.Pipe pairs so that gRPC stream messages flow into the MCP SDK's IOTransport.
//
// Data flow:
//
//	gRPC stream.Recv() -> pipeWriter (clientToServer) -> IOTransport.Reader -> MCP SDK
//	MCP SDK -> IOTransport.Writer -> pipeReader (serverToClient) -> gRPC stream.Send()
type GRPCTransport struct {
	stream    GRPCStream
	sessionID string
	// firstMsg is replayed into the pipe if the daemon already consumed it
	firstMsg *serenav1.MCPMessage
}

// NewGRPCTransport creates a new transport backed by a gRPC stream.
// If firstMsg is non-nil, it is replayed into the client->server pipe before reading from the stream.
func NewGRPCTransport(stream GRPCStream, sessionID string, firstMsg *serenav1.MCPMessage) *GRPCTransport {
	return &GRPCTransport{
		stream:    stream,
		sessionID: sessionID,
		firstMsg:  firstMsg,
	}
}

// Connect returns a Connection that bridges the gRPC stream to the MCP SDK.
// It spawns goroutines to pump data between gRPC and io.Pipe pairs.
func (t *GRPCTransport) Connect(ctx context.Context) (mcpsdk.Connection, error) {
	// Pipe for client-to-server: gRPC Recv -> pipeWriter, IOTransport reads from pipeReader
	clientReader, clientWriter := io.Pipe()
	// Pipe for server-to-client: IOTransport writes to pipeWriter, gRPC Send reads from pipeReader
	serverReader, serverWriter := io.Pipe()

	// Goroutine: gRPC stream.Recv() -> clientWriter (feeds IOTransport reader)
	go func() {
		defer clientWriter.Close()

		// Replay first message if present
		if t.firstMsg != nil {
			if _, err := clientWriter.Write(t.firstMsg.Payload); err != nil {
				clientWriter.CloseWithError(err)
				return
			}
			if _, err := clientWriter.Write([]byte("\n")); err != nil {
				clientWriter.CloseWithError(err)
				return
			}
		}

		for {
			msg, err := t.stream.Recv()
			if err != nil {
				clientWriter.CloseWithError(err)
				return
			}
			// Write payload as newline-delimited JSON (IOTransport expects this)
			if _, err := clientWriter.Write(msg.Payload); err != nil {
				return
			}
			if _, err := clientWriter.Write([]byte("\n")); err != nil {
				return
			}
		}
	}()

	// Goroutine: serverReader -> gRPC stream.Send() (reads IOTransport output)
	go func() {
		defer serverReader.Close()
		buf := make([]byte, 0, 64*1024)
		tmp := make([]byte, 4096)
		for {
			n, err := serverReader.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				// IOTransport writes newline-delimited JSON; split on newlines
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
							Payload:   line,
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

	// Create IOTransport with the pipe endpoints
	ioTransport := &mcpsdk.IOTransport{
		Reader: clientReader,
		Writer: serverWriter,
	}

	return ioTransport.Connect(ctx)
}
