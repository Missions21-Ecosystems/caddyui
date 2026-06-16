package handlers

import (
	"bytes"
	"caddyui/internal/stream"
	"caddyui/templates"
	"encoding/json"
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) PKIPage(w http.ResponseWriter, r *http.Request) {
	templates.PKIPage(stream.NewID()).Render(r.Context(), w)
}

func (h *Handler) PKICAInfoAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
		CAID     string `json:"caid"`
	}
	datastar.ReadSignals(r, &signals)
	if signals.CAID == "" {
		signals.CAID = "local"
	}
	info, err := h.caddy.GetPKICA(signals.CAID)
	if err != nil {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.PKIError("Failed to fetch CA info: " + err.Error()))
		})
	} else {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.PKICAContent(info))
		})
	}
	datastar.NewSSE(w, r)
}

func (h *Handler) PKICACertsAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
		CAID     string `json:"caid"`
	}
	datastar.ReadSignals(r, &signals)
	if signals.CAID == "" {
		signals.CAID = "local"
	}
	raw, err := h.caddy.GetPKICACerts(signals.CAID)
	if err != nil {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.PKIError("Failed to fetch certs: " + err.Error()))
		})
	} else {
		var buf bytes.Buffer
		json.Indent(&buf, raw, "", "  ")
		pretty := buf.String()
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.PKICertsContent(pretty))
		})
	}
	datastar.NewSSE(w, r)
}
