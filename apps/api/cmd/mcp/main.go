package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	transport := flag.String("transport", "stdio", "MCP transport to use (stdio, sse)")
	flag.Parse()

	fmt.Printf("Triage Model Context Protocol (MCP) Server (Phase 1 CLI)\n")
	fmt.Printf("Transport: %s\n", *transport)
	fmt.Printf("MCP tool registry adapter wired in Phase 19.\n")
	os.Exit(0)
}
