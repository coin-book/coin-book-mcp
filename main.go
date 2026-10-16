package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rotisserie/eris"

	"coin-book-mcp/internal/coinbook"
	"coin-book-mcp/internal/tools"
)

var version = "dev"

const (
	defaultAPIURL = "http://127.0.0.1:8082/api/v1"
	tokenPrefix   = "coinbook_pat_" //nolint:gosec // G101: a prefix, not a credential
)

// Stdout carries the MCP protocol, so every diagnostic goes to stderr.
func main() {
	log.SetFlags(0)
	log.SetPrefix("coin-book-mcp: ")
	log.SetOutput(os.Stderr)
	if err := run(); err != nil {
		log.Fatal(eris.ToString(err, false))
	}
}

func run() error {
	token := strings.TrimSpace(os.Getenv("COINBOOK_TOKEN"))
	if token == "" {
		return eris.New("COINBOOK_TOKEN is not set: create one in coin-book under Nastavení → Přístup pro AI agenta")
	}
	if !strings.HasPrefix(token, tokenPrefix) {
		return eris.New("COINBOOK_TOKEN does not look like a coin-book agent token (expected the coinbook_pat_ prefix)")
	}
	apiURL := strings.TrimSpace(os.Getenv("COINBOOK_API_URL"))
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "coin-book", Title: "coin-book finance", Version: version}, nil)
	tools.Register(server, coinbook.New(apiURL, token))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("serving %q over stdio (version %s)", apiURL, version) //nolint:gosec // G706: quoted, and the URL is the operator's own setting
	return server.Run(ctx, &mcp.StdioTransport{})
}
