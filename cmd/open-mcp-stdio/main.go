package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"well-ambient/internal/openmcp"
)

func main() {
	log.SetOutput(os.Stderr)
	invoker, err := openmcp.NewHTTPInvoker(
		os.Getenv("WELL_AMBIENT_BASE_URL"),
		os.Getenv("WELL_AMBIENT_API_KEY"),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := openmcp.NewServer(invoker).Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("MCP stdio server failed: %v", err)
	}
}
