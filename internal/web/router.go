package web

import (
	"caddyui/internal/auth"
	"caddyui/internal/caddy"
	"caddyui/internal/web/handlers"
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFiles embed.FS

func NewRouter(caddyClient *caddy.Client, username, password string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(auth.BasicAuth(username, password))

	h := handlers.New(caddyClient)

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
	r.Post("/api/pki/ca", h.PKICAInfoAction)
	r.Post("/api/pki/certs", h.PKICACertsAction)
	r.Post("/api/proxy/refresh", h.ProxyRefreshAction)
	r.Post("/api/routes/refresh", h.RoutesRefreshAction)
	r.Post("/api/routes/editor/open", h.RouteEditorOpenAction)
	r.Post("/api/routes/editor/close", h.RouteEditorCloseAction)
	r.Post("/api/routes/delete", h.RouteDeleteAction)
	r.Post("/api/routes/save", h.RouteSaveAction)
	r.Post("/api/server/stop", h.ServerStopAction)

	return r
}
