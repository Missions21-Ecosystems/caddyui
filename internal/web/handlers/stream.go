package handlers

import (
	"caddyui/internal/caddy"
	"caddyui/templates"
	"encoding/json"
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

// PageStreamSSE is the single long-lived SSE connection for a page.
// It registers the client, optionally pushes initial page state, then sits
// in a loop forwarding PushFns sent by action handlers via the stream manager.
func (h *Handler) PageStreamSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
		Page     string `json:"page"`
	}
	datastar.ReadSignals(r, &signals)

	if signals.StreamID == "" {
		http.Error(w, "missing streamId", http.StatusBadRequest)
		return
	}

	ch := h.streams.Register(signals.StreamID)
	defer h.streams.Unregister(signals.StreamID)

	sse := datastar.NewSSE(w, r)

	// Push initial content synchronously before entering the event loop.
	switch signals.Page {
	case "dashboard":
		h.pushDashboard(sse)
	case "proxy":
		h.pushProxyUpstreams(sse)
	case "routes":
		h.pushRoutes(sse)
	}

	for {
		select {
		case fn, ok := <-ch:
			if !ok {
				return
			}
			fn(sse)
		case <-r.Context().Done():
			return
		}
	}
}

// PingSSE is a one-shot SSE call used to check connectivity on page load.
func (h *Handler) PingSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(templates.ConnStatus(h.caddy.Ping()))
}

func (h *Handler) pushDashboard(sse *datastar.ServerSentEventGenerator) {
	online := h.caddy.Ping()
	var summary caddy.ConfigSummary
	var upstreams []caddy.Upstream
	if online {
		if raw, err := h.caddy.GetConfig(""); err == nil {
			summary = caddy.Summarize(raw)
		}
		upstreams, _ = h.caddy.GetUpstreams()
	}
	sse.PatchElementTempl(templates.DashboardStats(summary, upstreams, online))
	sse.PatchElementTempl(templates.ConnStatus(online))
}

func (h *Handler) pushProxyUpstreams(sse *datastar.ServerSentEventGenerator) {
	upstreams, _ := h.caddy.GetUpstreams()
	sse.PatchElementTempl(templates.ProxyContent(upstreams))
}

func (h *Handler) pushRoutes(sse *datastar.ServerSentEventGenerator) {
	raw, err := h.caddy.GetConfig("apps/http")
	if err != nil {
		sse.PatchElementTempl(templates.RoutesContent(caddy.HTTPApp{}))
		return
	}
	var app caddy.HTTPApp
	if err := json.Unmarshal(raw, &app); err != nil {
		sse.PatchElementTempl(templates.RoutesContent(caddy.HTTPApp{}))
		return
	}
	sse.PatchElementTempl(templates.RoutesContent(app))
}
