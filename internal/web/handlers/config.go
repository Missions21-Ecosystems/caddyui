package handlers

import (
	"bytes"
	"caddyui/internal/stream"
	"caddyui/templates"
	"encoding/json"
	"net/http"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) ConfigPage(w http.ResponseWriter, r *http.Request) {
	templates.ConfigPage(stream.NewID()).Render(r.Context(), w)
}

func (h *Handler) ConfigGetAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID   string `json:"streamid"`
		ConfigPath string `json:"configpath"`
	}
	datastar.ReadSignals(r, &signals)

	raw, err := h.caddy.GetConfig(signals.ConfigPath)
	if err != nil {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.ConfigStatus("Error fetching config: "+err.Error(), "error"))
		})
	} else {
		var buf bytes.Buffer
		json.Indent(&buf, raw, "", "  ")
		pretty := buf.String()
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.MarshalAndPatchSignals(map[string]any{"configjson": pretty})
			sse.PatchElementTempl(templates.ConfigStatus("Config fetched.", "success"))
		})
	}
	datastar.NewSSE(w, r)
}

func (h *Handler) ConfigLoadAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID   string `json:"streamid"`
		ConfigJson string `json:"configjson"`
	}
	datastar.ReadSignals(r, &signals)

	var pushFn stream.PushFn
	switch {
	case signals.ConfigJson == "":
		pushFn = func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.ConfigStatus("Config is empty — nothing to load.", "error"))
		}
	case !json.Valid(json.RawMessage(signals.ConfigJson)):
		pushFn = func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.ConfigStatus("Invalid JSON — fix syntax errors first.", "error"))
		}
	default:
		err := h.caddy.LoadConfig(json.RawMessage(signals.ConfigJson))
		if err != nil {
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.PatchElementTempl(templates.ConfigStatus("Caddy rejected config: "+err.Error(), "error"))
			}
		} else {
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.PatchElementTempl(templates.ConfigStatus("Config loaded successfully.", "success"))
				sse.PatchElementTempl(templates.ConnStatus(true))
			}
		}
	}
	h.streams.Push(signals.StreamID, pushFn)
	datastar.NewSSE(w, r)
}
