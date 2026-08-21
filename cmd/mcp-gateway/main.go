package main

import (
	"flag"
	"log"
	"os"

	"github.com/r9s-ai/mcphub/internal/gateway"
)

func main() {
	addr := flag.String("addr", ":3080", "listen address")
	token := flag.String("connect-token", os.Getenv("MCP_GATEWAY_CONNECT_TOKEN"), "shared token for mcp-connect registration")
	publicURL := flag.String("public-url", "http://127.0.0.1:3080", "public Gateway URL used in Device Code verification links")
	flag.Parse()
	log.Printf("mcp-gateway listening on %s", *addr)
	if *token == "" {
		log.Printf("connect token authentication disabled (development mode)")
	}
	log.Fatal(gateway.ListenAndServe(*addr, gateway.NewWithOptions(*token, *publicURL)))
}
