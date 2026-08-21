package main

import (
	"flag"
	"log"

	"github.com/r9s-ai/mcphub/internal/gateway"
)

func main() {
	addr := flag.String("addr", ":3080", "listen address")
	flag.Parse()
	log.Printf("mcp-gateway listening on %s", *addr)
	log.Fatal(gateway.ListenAndServe(*addr, gateway.New()))
}
