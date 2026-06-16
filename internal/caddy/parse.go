package caddy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MatchChip is a human-readable representation of one matcher condition.
type MatchChip struct {
	Kind string // "host", "path", "method", "ip", "header", "expr", "any", "other"
	Text string
}

// HandlerLine is a human-readable summary of one handler module.
type HandlerLine struct {
	Kind   string
	Detail string
}

// RouteDisplay is the fully parsed, display-ready form of an HTTPRoute.
type RouteDisplay struct {
	Num       int
	Chips     []MatchChip
	Handlers  []HandlerLine
	Terminal  bool
	SubRoutes []RouteDisplay
}

// ParseRoutes converts raw HTTPRoutes into RouteDisplay slices for the UI.
func ParseRoutes(routes []HTTPRoute) []RouteDisplay {
	out := make([]RouteDisplay, len(routes))
	for i, r := range routes {
		out[i] = parseRoute(i+1, r)
	}
	return out
}

func parseRoute(num int, r HTTPRoute) RouteDisplay {
	d := RouteDisplay{Num: num, Terminal: r.Terminal}

	for _, m := range r.Match {
		d.Chips = append(d.Chips, parseMatchChips(m)...)
	}
	if len(d.Chips) == 0 {
		d.Chips = []MatchChip{{Kind: "any", Text: "* all requests"}}
	}

	for _, h := range r.Handle {
		lines, subs := parseHandler(h)
		d.Handlers = append(d.Handlers, lines...)
		d.SubRoutes = append(d.SubRoutes, subs...)
	}
	return d
}

func parseMatchChips(raw json.RawMessage) []MatchChip {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return []MatchChip{{Kind: "other", Text: string(raw)}}
	}
	var chips []MatchChip
	for key, val := range m {
		switch key {
		case "host":
			var hosts []string
			json.Unmarshal(val, &hosts)
			chips = append(chips, MatchChip{Kind: "host", Text: strings.Join(hosts, ", ")})
		case "path":
			var paths []string
			json.Unmarshal(val, &paths)
			chips = append(chips, MatchChip{Kind: "path", Text: strings.Join(paths, ", ")})
		case "method":
			var methods []string
			json.Unmarshal(val, &methods)
			chips = append(chips, MatchChip{Kind: "method", Text: strings.Join(methods, " | ")})
		case "not":
			chips = append(chips, MatchChip{Kind: "not", Text: "NOT"})
		case "path_regexp":
			var re struct {
				Pattern string `json:"pattern"`
			}
			json.Unmarshal(val, &re)
			chips = append(chips, MatchChip{Kind: "regexp", Text: "~/" + re.Pattern + "/"})
		case "remote_ip":
			var ri struct {
				Ranges []string `json:"ranges"`
			}
			json.Unmarshal(val, &ri)
			chips = append(chips, MatchChip{Kind: "ip", Text: strings.Join(ri.Ranges, ", ")})
		case "header":
			chips = append(chips, MatchChip{Kind: "header", Text: "header"})
		case "header_regexp":
			chips = append(chips, MatchChip{Kind: "header", Text: "header~"})
		case "expression":
			var expr string
			if json.Unmarshal(val, &expr) == nil && len(expr) > 24 {
				expr = expr[:21] + "..."
			}
			chips = append(chips, MatchChip{Kind: "expr", Text: "expr: " + expr})
		default:
			chips = append(chips, MatchChip{Kind: "other", Text: key})
		}
	}
	return chips
}

func parseHandler(raw json.RawMessage) (lines []HandlerLine, subRoutes []RouteDisplay) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return []HandlerLine{{Kind: "?"}}, nil
	}
	var kind string
	json.Unmarshal(obj["handler"], &kind)

	switch kind {
	case "reverse_proxy":
		var rp struct {
			Upstreams []struct {
				Dial string `json:"dial"`
			} `json:"upstreams"`
		}
		json.Unmarshal(raw, &rp)
		dials := make([]string, len(rp.Upstreams))
		for i, u := range rp.Upstreams {
			dials[i] = u.Dial
		}
		lines = []HandlerLine{{Kind: "reverse_proxy", Detail: strings.Join(dials, ", ")}}

	case "file_server":
		var fs struct {
			Root string `json:"root"`
		}
		json.Unmarshal(raw, &fs)
		lines = []HandlerLine{{Kind: "file_server", Detail: fs.Root}}

	case "static_response":
		var sr struct {
			StatusCode json.RawMessage `json:"status_code"`
			Body       string          `json:"body"`
		}
		json.Unmarshal(raw, &sr)
		detail := strings.Trim(string(sr.StatusCode), `"`)
		if sr.Body != "" {
			b := sr.Body
			if len(b) > 24 {
				b = b[:21] + "..."
			}
			detail += ": " + b
		}
		lines = []HandlerLine{{Kind: "static_response", Detail: detail}}

	case "rewrite":
		var rw struct {
			URI              string `json:"uri"`
			StripPathPrefix  string `json:"strip_path_prefix"`
		}
		json.Unmarshal(raw, &rw)
		detail := rw.URI
		if detail == "" {
			detail = rw.StripPathPrefix
		}
		lines = []HandlerLine{{Kind: "rewrite", Detail: detail}}

	case "encode":
		lines = []HandlerLine{{Kind: "encode", Detail: "gzip"}}

	case "subroute":
		var sr struct {
			Routes []HTTPRoute `json:"routes"`
		}
		json.Unmarshal(raw, &sr)
		lines = []HandlerLine{{Kind: "subroute", Detail: fmt.Sprintf("%d route(s)", len(sr.Routes))}}
		for i, r := range sr.Routes {
			subRoutes = append(subRoutes, parseRoute(i+1, r))
		}

	case "php_fastcgi":
		var pf struct {
			Upstreams []struct {
				Dial string `json:"dial"`
			} `json:"upstreams"`
		}
		json.Unmarshal(raw, &pf)
		dials := make([]string, len(pf.Upstreams))
		for i, u := range pf.Upstreams {
			dials[i] = u.Dial
		}
		lines = []HandlerLine{{Kind: "php_fastcgi", Detail: strings.Join(dials, ", ")}}

	default:
		if kind == "" {
			kind = "handler"
		}
		lines = []HandlerLine{{Kind: kind}}
	}
	return
}
