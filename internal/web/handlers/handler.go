package handlers

import (
	"caddyui/internal/caddy"
	"caddyui/internal/stream"
)

type Handler struct {
	caddy   *caddy.Client
	streams *stream.Manager
}

func New(c *caddy.Client) *Handler {
	return &Handler{
		caddy:   c,
		streams: stream.NewManager(),
	}
}
