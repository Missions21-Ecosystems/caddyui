package handlers

import (
	"caddyui/internal/stream"
	"caddyui/templates"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) RoutesPage(w http.ResponseWriter, r *http.Request) {
	templates.RoutesPage(stream.NewID()).Render(r.Context(), w)
}

func (h *Handler) RoutesRefreshAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
	}
	datastar.ReadSignals(r, &signals)
	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		h.pushRoutes(sse)
	})
	datastar.NewSSE(w, r)
}

func (h *Handler) RouteEditorOpenAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
	}
	datastar.ReadSignals(r, &signals)
	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		sse.PatchElementTempl(templates.RouteEditorPanel())
	})
	datastar.NewSSE(w, r)
}

func (h *Handler) RouteEditorCloseAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID string `json:"streamid"`
	}
	datastar.ReadSignals(r, &signals)
	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		sse.PatchElementTempl(templates.RouteEditorClosed())
	})
	datastar.NewSSE(w, r)
}

func (h *Handler) RouteDeleteAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID       string `json:"streamid"`
		EditServer     string `json:"editserver"`
		EditRouteIndex string `json:"editrouteindex"`
	}
	datastar.ReadSignals(r, &signals)

	idx, err := strconv.Atoi(signals.EditRouteIndex)
	if err != nil {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.Toast("Invalid route index.", "error"))
		})
		datastar.NewSSE(w, r)
		return
	}

	path := fmt.Sprintf("apps/http/servers/%s/routes/%d", signals.EditServer, idx)
	deleteErr := h.caddy.DeleteConfig(path)

	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		if deleteErr != nil {
			sse.PatchElementTempl(templates.Toast("Delete failed: "+deleteErr.Error(), "error"))
		} else {
			h.pushRoutes(sse)
			sse.PatchElementTempl(templates.RouteEditorClosed())
			sse.PatchElementTempl(templates.Toast("Route deleted.", "success"))
		}
	})
	datastar.NewSSE(w, r)
}

func (h *Handler) RouteSaveAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID       string `json:"streamid"`
		EditServer     string `json:"editserver"`
		EditRouteIndex string `json:"editrouteindex"`
		RouteJSON      string `json:"routejson"`
	}
	datastar.ReadSignals(r, &signals)

	idx, err := strconv.Atoi(signals.EditRouteIndex)
	if err != nil {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.Toast("Invalid route index.", "error"))
		})
		datastar.NewSSE(w, r)
		return
	}

	if !json.Valid([]byte(signals.RouteJSON)) {
		h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.Toast("Invalid JSON — fix the syntax and try again.", "error"))
		})
		datastar.NewSSE(w, r)
		return
	}

	path := fmt.Sprintf("apps/http/servers/%s/routes/%d", signals.EditServer, idx)
	saveErr := h.caddy.SetConfig(path, json.RawMessage(signals.RouteJSON))

	h.streams.Push(signals.StreamID, func(sse *datastar.ServerSentEventGenerator) {
		if saveErr != nil {
			sse.PatchElementTempl(templates.Toast("Save failed: "+saveErr.Error(), "error"))
		} else {
			h.pushRoutes(sse)
			sse.PatchElementTempl(templates.RouteEditorClosed())
			sse.PatchElementTempl(templates.Toast("Route saved.", "success"))
		}
	})
	datastar.NewSSE(w, r)
}
