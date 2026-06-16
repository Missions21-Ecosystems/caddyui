package main

import (
	"caddyui/internal/caddy"
	"caddyui/internal/web"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present (ignored if not found).
	_ = godotenv.Load()

	caddyURL := getenv("CADDY_ADMIN_URL", "http://localhost:2019")
	listenAddr := getenv("LISTEN_ADDR", ":8080")
	username := getenv("UI_USERNAME", "admin")
	password := getenv("UI_PASSWORD", "changeme")
	basePath := strings.TrimRight(getenv("BASE_PATH", ""), "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	caddyClient := caddy.New(caddyURL)

	router := web.NewRouter(caddyClient, username, password, basePath)

	fmt.Printf("Caddy UI listening on %s  →  Caddy admin: %s\n", listenAddr, caddyURL)
	if err := http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
