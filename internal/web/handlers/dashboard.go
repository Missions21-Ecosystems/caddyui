package handlers

import (
	"caddyui/internal/stream"
	"caddyui/templates"
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	templates.DashboardPage(stream.NewID()).Render(r.Context(), w)
}

func (h *Handler) DashboardRefreshAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
	}
	datastar.ReadSignals(r, &signals)
	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		h.pushDashboard(sse)
	})
	datastar.NewSSE(w, r)
}
