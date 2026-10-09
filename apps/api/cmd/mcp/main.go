package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/mcp"
	"github.com/seraph/triage/apps/api/internal/tools"
)

func main() {
	transport := flag.String("transport", "stdio", "MCP transport to use ('stdio' or 'sse')")
	addr := flag.String("addr", ":8081", "Address to bind for SSE transport (e.g. :8081 or 127.0.0.1:8081)")
	orgIDStr := flag.String("org-id", "00000000-0000-0000-0000-000000000001", "Tenant organization UUID context")
	flag.Parse()

	var orgID pgtype.UUID
	if err := orgID.Scan(*orgIDStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid org-id UUID %q: %v\n", *orgIDStr, err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With(
		slog.String("service", "triage-mcp"),
	)

	// 1. Initialize diagnostic tool registry with all 6 read-only tools
	policyProvider := tools.NewAllowAllPolicyProvider()
	toolReg := tools.NewToolRegistry(policyProvider, 20*time.Second)

	_ = toolReg.Register(tools.NewLogSearchTool(nil))
	_ = toolReg.Register(tools.NewRecentDeploysTool(nil))
	_ = toolReg.Register(tools.NewServiceTopologyTool(tools.NewInMemoryTopologyProvider()))
	_ = toolReg.Register(tools.NewQueryMetricsTool(nil))
	_ = toolReg.Register(tools.NewSearchRunbooksTool(nil))
	_ = toolReg.Register(tools.NewSimilarIncidentsTool(nil))

	// 2. Initialize MCP server
	server := mcp.NewServer(toolReg, nil, orgID, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch *transport {
	case "stdio":
		logger.Info("starting triage Model Context Protocol (MCP) server over stdio", "org_id", *orgIDStr)
		if err := mcp.ServeStdio(ctx, server, os.Stdin, os.Stdout); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "MCP stdio server error: %v\n", err)
			os.Exit(1)
		}

	case "sse":
		logger.Info("starting triage Model Context Protocol (MCP) server over SSE", "addr", *addr, "org_id", *orgIDStr)
		if err := mcp.ServeSSE(ctx, server, *addr); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "MCP SSE server error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported transport %q (supported: 'stdio', 'sse')\n", *transport)
		os.Exit(1)
	}
}
