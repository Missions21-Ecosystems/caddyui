package web

import (
	"caddyui/internal/auth"
	"caddyui/internal/basepath"
	"caddyui/internal/caddy"
	"caddyui/internal/web/handlers"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFiles embed.FS

func NewRouter(caddyClient *caddy.Client, username, password, bp string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(basepath.WithBasePath(r.Context(), bp)))
		})
	})

	h := handlers.New(caddyClient)

	// /logout must be outside Basic Auth so the browser sees the 401 and
	// clears its cached credentials, re-prompting on the next navigation.
	r.Get("/logout", logoutHandler(bp))

	// Everything else requires authentication.
	r.Group(func(r chi.Router) {
		r.Use(auth.BasicAuth(username, password))

		staticFS, _ := fs.Sub(staticFiles, "static")
		r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

		// Pages
		r.Get("/", h.DashboardPage)
		r.Get("/config", h.ConfigPage)
		r.Get("/routes", h.RoutesPage)
		r.Get("/proxy", h.ProxyPage)
		r.Get("/pki", h.PKIPage)
		r.Get("/adapt", h.AdaptPage)

		// Long-lived SSE stream (one per page, receives all server-push updates)
		r.Get("/sse/stream", h.PageStreamSSE)
		// One-shot ping for connection status badge
		r.Get("/sse/ping", h.PingSSE)

		// Short-lived action endpoints — push results to the page stream
		r.Post("/api/config/get", h.ConfigGetAction)
		r.Post("/api/config/load", h.ConfigLoadAction)
		r.Post("/api/dashboard/refresh", h.DashboardRefreshAction)
		r.Post("/api/adapt", h.AdaptAction)
		r.Post("/api/adapt/merge", h.AdaptMergeAction)
		r.Post("/api/pki/ca", h.PKICAInfoAction)
		r.Post("/api/pki/certs", h.PKICACertsAction)
		r.Post("/api/proxy/refresh", h.ProxyRefreshAction)
		r.Post("/api/routes/refresh", h.RoutesRefreshAction)
		r.Post("/api/routes/editor/open", h.RouteEditorOpenAction)
		r.Post("/api/routes/editor/close", h.RouteEditorCloseAction)
		r.Post("/api/routes/delete", h.RouteDeleteAction)
		r.Post("/api/routes/save", h.RouteSaveAction)
		r.Post("/api/server/stop", h.ServerStopAction)
		r.Post("/api/config/caddyfile", h.ConfigToCaddyfileAction)
	})

	return r
}

// logoutHandler clears the browser's cached Basic Auth credentials.
//
// Returning 401+WWW-Authenticate from an unprotected route causes the browser
// to show its native credential dialog and loop. Instead we return 200 and use
// a synchronous XHR with deliberately wrong credentials against a protected
// endpoint — the 401 from the auth middleware flushes the credential cache in
// most browsers — then redirect to "/" which will re-prompt for login.
func logoutHandler(bp string) http.HandlerFunc {
	root := bp + "/"
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="UTF-8">
<title>Logging out…</title>
<style>body{font-family:sans-serif;background:#0d1117;color:#e6edf3;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}</style>
</head><body><p>Logging out…</p>
<script>
try {
  var x = new XMLHttpRequest();
  x.open('GET', '%s', false, '__logout__', '__' + Math.random());
  x.send();
} catch(e) {}
window.location.replace('%s');
</script>
<noscript><a href="%s" style="color:#388bfd">Log in</a></noscript>
</body></html>`, root, root, root)
	}
}
