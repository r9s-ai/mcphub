package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/r9s-ai/mcphub/internal/gateway"
	"github.com/r9s-ai/mcphub/internal/store/postgres"
	redisstore "github.com/r9s-ai/mcphub/internal/store/redis"
)

func main() {
	addr := flag.String("addr", ":3080", "listen address")
	token := flag.String("connect-token", os.Getenv("MCP_GATEWAY_CONNECT_TOKEN"), "shared token for mcp-connect registration")
	publicURL := flag.String("public-url", "http://127.0.0.1:3080", "public Gateway URL used in Device Code verification links")
	migrateOnly := flag.Bool("migrate-only", false, "run PostgreSQL migrations and exit")
	flag.Parse()
	log.Printf("mcp-gateway listening on %s", *addr)
	if *token == "" {
		log.Printf("connect token authentication disabled (development mode)")
	}
	storage := os.Getenv("MCP_STORAGE")
	var connectStore *postgres.ConnectStore
	if strings.EqualFold(storage, "postgres") || os.Getenv("DATABASE_URL") != "" {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL is required when MCP_STORAGE=postgres")
		}
		if err := postgres.Migrate(context.Background(), dsn, getenv("MCP_MIGRATIONS_DIR", "migrations")); err != nil {
			log.Fatal(err)
		}
		var err error
		connectStore, err = postgres.Open(context.Background(), dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer connectStore.Close()
	}
	if *migrateOnly {
		if connectStore == nil {
			log.Fatal("MCP_STORAGE=postgres or DATABASE_URL is required")
		}
		return
	}
	var presence *redisstore.PresenceStore
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" && strings.ToLower(os.Getenv("MCP_REDIS_ENABLED")) != "false" {
		var err error
		presence, err = redisstore.New(redisURL)
		if err != nil {
			log.Fatal(err)
		}
		defer presence.Close()
	}
	log.Fatal(gateway.ListenAndServe(*addr, gateway.NewWithStores(*token, *publicURL, connectStore, presence, connectStore)))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
