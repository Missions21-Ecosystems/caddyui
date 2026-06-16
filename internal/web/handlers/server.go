package handlers

import (
	"caddyui/templates"
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) ServerStopAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
	}
	datastar.ReadSignals(r, &signals)
	err := h.caddy.StopServer()
	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		if err != nil {
			sse.PatchElementTempl(templates.Toast("Failed to stop Caddy: "+err.Error(), "error"))
		} else {
			sse.PatchElementTempl(templates.ConnStatus(false))
			sse.PatchElementTempl(templates.Toast("Caddy server stopped.", "warn"))
		}
	})
	datastar.NewSSE(w, r)
}
