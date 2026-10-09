package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ServeStdio processes JSON-RPC messages from standard input and writes responses to standard output.
func ServeStdio(ctx context.Context, server *Server, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow large messages up to 10MB
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			errResp := &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      nil,
				Error: &JSONRPCError{
					Code:    CodeParseError,
					Message: fmt.Sprintf("invalid JSON payload: %v", err),
				},
			}
			mu.Lock()
			bytes, _ := json.Marshal(errResp)
			_, _ = fmt.Fprintf(out, "%s\n", string(bytes))
			mu.Unlock()
			continue
		}

		resp := server.HandleRequest(ctx, req)
		if resp != nil {
			bytes, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			mu.Lock()
			_, _ = fmt.Fprintf(out, "%s\n", string(bytes))
			mu.Unlock()
		}
	}

	return scanner.Err()
}
