package caddyfile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FromJSON converts a Caddy JSON config to an equivalent Caddyfile.
func FromJSON(raw json.RawMessage) (string, error) {
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	var b strings.Builder

	// Global options block — only when admin listen differs from the default
	if adminMap := asMap(cfg["admin"]); adminMap != nil {
		if listen := strVal(adminMap, "listen"); listen != "" && listen != "localhost:2019" {
			b.WriteString("{\n\tadmin ")
			b.WriteString(listen)
			b.WriteString("\n}\n\n")
		}
	}

	// HTTP servers
	httpMap := asMap(nestedMap(cfg, "apps", "http"))
	if httpMap == nil {
		return b.String(), nil
	}
	servers := asMap(httpMap["servers"])
	if servers == nil {
		return b.String(), nil
	}

	for _, name := range sortedKeys(servers) {
		srv := asMap(servers[name])
		if srv == nil {
			continue
		}
		listenPorts := strSlice(srv, "listen")
		routes := anySlice(srv, "routes")
		writeTopRoutes(&b, routes, listenPorts)
	}

	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return "", nil
	}
	return out + "\n", nil
}

// writeTopRoutes writes each server-level route as a Caddyfile site block.
func writeTopRoutes(b *strings.Builder, routes []any, listenPorts []string) {
	for _, rv := range routes {
		route := asMap(rv)
		if route == nil {
			continue
		}
		writeSiteBlock(b, route, listenPorts)
	}
}

// writeSiteBlock renders one top-level route as a `hostname { }` site block.
func writeSiteBlock(b *strings.Builder, route map[string]any, listenPorts []string) {
	hosts := extractHosts(route)
	var addr string
	if len(hosts) > 0 {
		addr = strings.Join(hosts, ", ")
	} else if len(listenPorts) > 0 {
		addr = listenPorts[0]
	} else {
		addr = ":80"
	}

	b.WriteString(addr)
	b.WriteString(" {\n")

	// Top-level handle is almost always a single subroute wrapper — unwrap it.
	for _, hv := range anySlice(route, "handle") {
		h := asMap(hv)
		if h == nil {
			continue
		}
		if strVal(h, "handler") == "subroute" {
			writeSiteBodyRoutes(b, anySlice(h, "routes"), 1)
		} else {
			writeHandlerDirective(b, h, 1)
		}
	}

	b.WriteString("}\n\n")
}

// writeSiteBodyRoutes writes the routes inside a site block,
// collapsing path-matched routes into handle_path / handle directives.
func writeSiteBodyRoutes(b *strings.Builder, routes []any, depth int) {
	for _, rv := range routes {
		route := asMap(rv)
		if route == nil {
			continue
		}

		paths := extractPaths(route)
		if len(paths) > 0 {
			if matchPath, bodyRoutes, ok := detectHandlePath(route); ok {
				b.WriteString(tabs(depth))
				b.WriteString("handle_path ")
				b.WriteString(matchPath)
				b.WriteString(" {\n")
				writeBodyRoutes(b, bodyRoutes, depth+1)
				b.WriteString(tabs(depth))
				b.WriteString("}\n")
			} else {
				b.WriteString(tabs(depth))
				b.WriteString("handle ")
				b.WriteString(strings.Join(paths, " "))
				b.WriteString(" {\n")
				writeRouteHandlers(b, route, depth+1)
				b.WriteString(tabs(depth))
				b.WriteString("}\n")
			}
		} else {
			writeRouteHandlers(b, route, depth)
		}
	}
}

// writeBodyRoutes writes routes that appear inside a handle_path / handle block.
func writeBodyRoutes(b *strings.Builder, routes []any, depth int) {
	for _, rv := range routes {
		route := asMap(rv)
		if route == nil {
			continue
		}
		writeRouteHandlers(b, route, depth)
	}
}

// writeRouteHandlers writes the handlers of a single route, unwrapping subroutes transparently.
func writeRouteHandlers(b *strings.Builder, route map[string]any, depth int) {
	for _, hv := range anySlice(route, "handle") {
		h := asMap(hv)
		if h == nil {
			continue
		}
		if strVal(h, "handler") == "subroute" {
			writeBodyRoutes(b, anySlice(h, "routes"), depth)
		} else {
			writeHandlerDirective(b, h, depth)
		}
	}
}

// writeHandlerDirective converts one Caddy handler map to a Caddyfile directive line.
func writeHandlerDirective(b *strings.Builder, h map[string]any, depth int) {
	switch strVal(h, "handler") {
	case "reverse_proxy":
		writeReverseProxy(b, h, depth)
	case "static_response":
		writeStaticResponse(b, h, depth)
	case "authentication":
		writeAuthentication(b, h, depth)
	case "rewrite":
		writeRewrite(b, h, depth)
	default:
		if name := strVal(h, "handler"); name != "" {
			b.WriteString(tabs(depth))
			b.WriteString("# ")
			b.WriteString(name)
			b.WriteString(" (not converted)\n")
		}
	}
}

func writeReverseProxy(b *strings.Builder, h map[string]any, depth int) {
	upstreams := anySlice(h, "upstreams")
	if len(upstreams) == 0 {
		return
	}

	tlsInsecure := false
	if transport := asMap(h["transport"]); transport != nil {
		if tlsMap := asMap(transport["tls"]); tlsMap != nil {
			if v, _ := tlsMap["insecure_skip_verify"].(bool); v {
				tlsInsecure = true
			}
		}
	}

	var dials []string
	for _, uv := range upstreams {
		u := asMap(uv)
		if u == nil {
			continue
		}
		dial := strVal(u, "dial")
		if tlsInsecure && !strings.HasPrefix(dial, "https://") {
			dial = "https://" + dial
		}
		dials = append(dials, dial)
	}

	b.WriteString(tabs(depth))
	b.WriteString("reverse_proxy ")
	b.WriteString(strings.Join(dials, " "))

	if tlsInsecure {
		b.WriteString(" {\n")
		b.WriteString(tabs(depth + 1))
		b.WriteString("transport http {\n")
		b.WriteString(tabs(depth + 2))
		b.WriteString("tls_insecure_skip_verify\n")
		b.WriteString(tabs(depth + 1))
		b.WriteString("}\n")
		b.WriteString(tabs(depth))
		b.WriteString("}\n")
	} else {
		b.WriteString("\n")
	}
}

func writeStaticResponse(b *strings.Builder, h map[string]any, depth int) {
	headers := asMap(h["headers"])
	status := intVal(h, "status_code")

	// Redirect pattern: Location header + 3xx status
	if headers != nil && status >= 300 && status < 400 {
		if locs, ok := headers["Location"].([]any); ok && len(locs) > 0 {
			loc := normalizePlaceholders(fmt.Sprintf("%v", locs[0]))
			b.WriteString(tabs(depth))
			b.WriteString("redir ")
			b.WriteString(loc)
			b.WriteString(fmt.Sprintf(" %d\n", status))
			return
		}
	}

	// Generic static response
	b.WriteString(tabs(depth))
	b.WriteString("respond")
	if body := strVal(h, "body"); body != "" {
		b.WriteString(fmt.Sprintf(" %q", body))
	}
	if status > 0 {
		b.WriteString(fmt.Sprintf(" %d", status))
	}
	b.WriteString("\n")
}

func writeAuthentication(b *strings.Builder, h map[string]any, depth int) {
	providers := asMap(h["providers"])
	if providers == nil {
		return
	}
	httpBasic := asMap(providers["http_basic"])
	if httpBasic == nil {
		return
	}

	algorithm := "bcrypt"
	if hashMap := asMap(httpBasic["hash"]); hashMap != nil {
		if alg := strVal(hashMap, "algorithm"); alg != "" {
			algorithm = alg
		}
	}

	b.WriteString(tabs(depth))
	b.WriteString("basic_auth ")
	b.WriteString(algorithm)
	b.WriteString(" {\n")
	for _, av := range anySlice(httpBasic, "accounts") {
		acc := asMap(av)
		if acc == nil {
			continue
		}
		b.WriteString(tabs(depth + 1))
		b.WriteString(strVal(acc, "username"))
		b.WriteString(" ")
		b.WriteString(strVal(acc, "password"))
		b.WriteString("\n")
	}
	b.WriteString(tabs(depth))
	b.WriteString("}\n")
}

func writeRewrite(b *strings.Builder, h map[string]any, depth int) {
	// Empty rewrite is a no-op artifact from Caddyfile → JSON round-trips; skip it.
	uri := strVal(h, "uri")
	strip := strVal(h, "strip_path_prefix")
	if uri == "" && strip == "" {
		return
	}
	if uri != "" {
		b.WriteString(tabs(depth))
		b.WriteString("rewrite * ")
		b.WriteString(normalizePlaceholders(uri))
		b.WriteString("\n")
	}
	// strip_path_prefix is consumed by detectHandlePath and should not reach here,
	// but emit as a comment if it does.
	if strip != "" && uri == "" {
		b.WriteString(tabs(depth))
		b.WriteString("# strip_path_prefix ")
		b.WriteString(strip)
		b.WriteString("\n")
	}
}

// detectHandlePath returns true when a route matches the handle_path pattern:
// path match + subroute whose first inner route is a strip_path_prefix rewrite.
// It returns the match path and the body routes (inner routes after the strip rewrite).
func detectHandlePath(route map[string]any) (matchPath string, bodyRoutes []any, ok bool) {
	paths := extractPaths(route)
	if len(paths) != 1 {
		return "", nil, false
	}

	handlers := anySlice(route, "handle")
	if len(handlers) != 1 {
		return "", nil, false
	}
	outerH := asMap(handlers[0])
	if strVal(outerH, "handler") != "subroute" {
		return "", nil, false
	}

	inner := anySlice(outerH, "routes")
	if len(inner) == 0 {
		return "", nil, false
	}

	// First inner route must be a single strip_path_prefix rewrite
	first := asMap(inner[0])
	if first == nil {
		return "", nil, false
	}
	firstHandlers := anySlice(first, "handle")
	if len(firstHandlers) != 1 {
		return "", nil, false
	}
	firstH := asMap(firstHandlers[0])
	if strVal(firstH, "handler") != "rewrite" || strVal(firstH, "strip_path_prefix") == "" {
		return "", nil, false
	}

	return paths[0], inner[1:], true
}

// --- helpers ---

func extractHosts(route map[string]any) []string {
	for _, mv := range anySlice(route, "match") {
		m := asMap(mv)
		if m == nil {
			continue
		}
		if hosts := strSlice(m, "host"); len(hosts) > 0 {
			return hosts
		}
	}
	return nil
}

func extractPaths(route map[string]any) []string {
	for _, mv := range anySlice(route, "match") {
		m := asMap(mv)
		if m == nil {
			continue
		}
		if paths := strSlice(m, "path"); len(paths) > 0 {
			return paths
		}
	}
	return nil
}

func normalizePlaceholders(s string) string {
	s = strings.ReplaceAll(s, "{http.request.uri}", "{uri}")
	s = strings.ReplaceAll(s, "{http.request.uri.path}", "{path}")
	return s
}

func tabs(n int) string { return strings.Repeat("\t", n) }

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func strVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func intVal(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func strSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func anySlice(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	s, _ := m[key].([]any)
	return s
}

func nestedMap(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		cur = asMap(cur[k])
		if cur == nil {
			return nil
		}
	}
	return cur
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
