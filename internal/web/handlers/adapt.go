package handlers

import (
	"bytes"
	"caddyui/internal/stream"
	"caddyui/templates"
	"encoding/json"
	"net/http"
	"strings"

	datastar "github.com/starfederation/datastar-go/datastar"
)

func (h *Handler) AdaptMergeAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID   string `json:"streamid"`
		ConfigJSON string `json:"configjson"`
	}
	datastar.ReadSignals(r, &signals)

	var pushFn stream.PushFn
	if signals.ConfigJSON == "" {
		pushFn = func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.Toast("Nothing to merge — convert a Caddyfile first.", "error"))
		}
	} else {
		err := h.caddy.MergeConfig([]byte(signals.ConfigJSON))
		if err != nil {
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.PatchElementTempl(templates.Toast("Merge failed: "+err.Error(), "error"))
			}
		} else {
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.PatchElementTempl(templates.Toast("Config merged successfully.", "success"))
			}
		}
	}
	h.streams.Push(signals.StreamID, pushFn)
	datastar.NewSSE(w, r)
}

func (h *Handler) AdaptPage(w http.ResponseWriter, r *http.Request) {
	templates.AdaptPage(stream.NewID()).Render(r.Context(), w)
}

func (h *Handler) AdaptAction(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StreamID  string `json:"streamid"`
		Caddyfile string `json:"caddyfile"`
	}
	datastar.ReadSignals(r, &signals)

	var pushFn stream.PushFn
	if signals.Caddyfile == "" {
		pushFn = func(sse *datastar.ServerSentEventGenerator) {
			sse.PatchElementTempl(templates.AdaptErrorOutput("Caddyfile input is empty."))
			sse.PatchElementTempl(templates.AdaptErrorActions())
		}
	} else {
		input := normalizeCaddyfile(signals.Caddyfile)
		raw, err := h.caddy.AdaptConfig(input, "caddyfile")
		if err != nil {
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.PatchElementTempl(templates.AdaptErrorOutput("Adaptation failed: " + err.Error()))
				sse.PatchElementTempl(templates.AdaptErrorActions())
			}
		} else {
			// Unwrap Caddy's {"warnings":[...],"result":{...}} envelope.
			var envelope struct {
				Warnings []struct {
					Message string `json:"message"`
				} `json:"warnings"`
				Result json.RawMessage `json:"result"`
			}
			configJSON := raw
			var warnings []string
			if json.Unmarshal(raw, &envelope) == nil && envelope.Result != nil {
				configJSON = envelope.Result
				for _, w := range envelope.Warnings {
					// Suppress the pure-formatting lint warning — it doesn't
					// affect the output and is meaningless in a web UI context.
					if strings.Contains(w.Message, "not formatted") {
						continue
					}
					warnings = append(warnings, w.Message)
				}
			}
			var buf bytes.Buffer
			json.Indent(&buf, configJSON, "", "  ")
			pretty := buf.String()
			w2 := warnings // capture
			pushFn = func(sse *datastar.ServerSentEventGenerator) {
				sse.MarshalAndPatchSignals(map[string]any{"configjson": pretty})
				sse.PatchElementTempl(templates.AdaptOutput(pretty, w2))
				sse.PatchElementTempl(templates.AdaptActions())
			}
		}
	}
	h.streams.Push(signals.StreamID, pushFn)
	datastar.NewSSE(w, r)
}

// normalizeCaddyfile converts leading spaces to tabs so Caddy's formatter
// doesn't emit the "not formatted" lint warning for indentation style.
func normalizeCaddyfile(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		// Count leading spaces and replace every group of 2 or 4 with a tab.
		stripped := strings.TrimLeft(line, " ")
		spaces := len(line) - len(stripped)
		if spaces == 0 {
			continue
		}
		// Determine indent width from the first indented line we see.
		tabs := spacesToTabs(line)
		lines[i] = tabs + stripped
	}
	return strings.Join(lines, "\n")
}

// spacesToTabs replaces the leading whitespace of a line with tabs.
// It tries indent widths of 4, 3, and 2 spaces (in that order) and
// falls back to one tab per space-run.
func spacesToTabs(line string) string {
	stripped := strings.TrimLeft(line, " ")
	spaces := len(line) - len(stripped)
	if spaces == 0 {
		return ""
	}
	for _, width := range []int{4, 3, 2} {
		if spaces%width == 0 {
			return strings.Repeat("\t", spaces/width)
		}
	}
	return "\t" // single tab for any odd count
}
