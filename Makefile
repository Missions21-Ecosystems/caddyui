.DEFAULT_GOAL := help

TEMPL_VERSION := v0.3.1020
BINARY        := caddyui

# ── Help ─────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Setup ─────────────────────────────────────────────────────────────────────

.PHONY: install-tools
install-tools: ## Install templ code generator
	go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)

.PHONY: setup
setup: install-tools ## Install tools and download Go dependencies
	go mod download

# ── Generate & build ──────────────────────────────────────────────────────────

.PHONY: generate
generate: ## Regenerate Go code from .templ files (run after editing any template)
	templ generate

.PHONY: build
build: generate ## Generate templates and build the binary
	go build -o $(BINARY) ./cmd/

.PHONY: run
run: generate ## Generate templates and run the UI locally (needs a running Caddy at CADDY_ADMIN_URL)
	go run ./cmd/

# ── Docker ───────────────────────────────────────────────────────────────────

.PHONY: up
up: ## Build Docker images and start all services in the background
	docker compose up --build -d

.PHONY: down
down: ## Stop and remove all containers
	docker compose down

.PHONY: logs
logs: ## Tail logs from all containers (Ctrl-C to stop)
	docker compose logs -f

.PHONY: restart
restart: ## Restart only the caddyui service (picks up a new binary without touching Caddy)
	docker compose restart caddyui

.PHONY: rebuild
rebuild: ## Force a full Docker rebuild with no layer cache (use after dependency changes)
	docker compose build --no-cache caddyui
	docker compose up -d

# ── Caddy helpers ─────────────────────────────────────────────────────────────

.PHONY: hash-password
hash-password: ## Generate a bcrypt password hash for Caddy basic_auth (usage: make hash-password PW=secret)
	docker compose exec caddy caddy hash-password --plaintext "$(PW)"

.PHONY: caddy-restart
caddy-restart: ## Restart the Caddy container (e.g. after stopping it from the UI)
	docker compose restart caddy

.PHONY: caddy-fmt
caddy-fmt: ## Format a Caddyfile in-place via the Caddy container (usage: make caddy-fmt FILE=./conf/Caddyfile)
	docker compose exec caddy caddy fmt --overwrite $(FILE)

# ── Quality ───────────────────────────────────────────────────────────────────

.PHONY: vet
vet: generate ## Run go vet (static analysis)
	go vet ./...

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

.PHONY: clean
clean: ## Remove the built binary and generated *_templ.go files
	rm -f $(BINARY)
	find . -name '*_templ.go' -not -path '*/vendor/*' -delete
