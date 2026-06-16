# ── Stage 1: generate templ + build binary ──────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Install templ code generator
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate templ components
RUN templ generate

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /caddyui ./cmd/

# ── Stage 2: minimal runtime image ──────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /caddyui /caddyui

EXPOSE 8080

ENTRYPOINT ["/caddyui"]
