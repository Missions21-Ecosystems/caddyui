package caddy

import "encoding/json"

// Config is the top-level Caddy configuration.
type Config struct {
	Admin   *AdminConfig               `json:"admin,omitempty"`
	Apps    map[string]json.RawMessage `json:"apps,omitempty"`
	Logging json.RawMessage            `json:"logging,omitempty"`
}

// AdminConfig is the Caddy admin listener config.
type AdminConfig struct {
	Listen   string `json:"listen,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// HTTPApp is the parsed structure of apps.http.
type HTTPApp struct {
	Servers map[string]HTTPServer `json:"servers,omitempty"`
}

// HTTPServer is a single HTTP server in the Caddy config.
type HTTPServer struct {
	Listen []string    `json:"listen,omitempty"`
	Routes []HTTPRoute `json:"routes,omitempty"`
}

// HTTPRoute is a route within an HTTP server.
type HTTPRoute struct {
	Match    []json.RawMessage `json:"match,omitempty"`
	Handle   []json.RawMessage `json:"handle,omitempty"`
	Terminal bool              `json:"terminal,omitempty"`
}

// Upstream is a reverse proxy upstream as returned by /reverse_proxy/upstreams.
type Upstream struct {
	Address     string `json:"address"`
	NumRequests int    `json:"num_requests"`
	Fails       int    `json:"fails"`
}

// PKICAInfo is the response from /pki/ca/<id>.
type PKICAInfo struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	RootCertificate         string `json:"root_certificate"`
	IntermediateCertificate string `json:"intermediate_certificate"`
}

// ConfigSummary is derived from a loaded Config for display purposes.
type ConfigSummary struct {
	Apps        []string
	ServerCount int
	RouteCount  int
	AdminListen string
}

// Summarize extracts human-readable stats from a raw config JSON.
func Summarize(raw json.RawMessage) ConfigSummary {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ConfigSummary{}
	}

	s := ConfigSummary{}
	if cfg.Admin != nil && cfg.Admin.Listen != "" {
		s.AdminListen = cfg.Admin.Listen
	}

	for appName, appRaw := range cfg.Apps {
		s.Apps = append(s.Apps, appName)
		if appName == "http" {
			var httpApp HTTPApp
			if err := json.Unmarshal(appRaw, &httpApp); err == nil {
				s.ServerCount = len(httpApp.Servers)
				for _, srv := range httpApp.Servers {
					s.RouteCount += len(srv.Routes)
				}
			}
		}
	}
	return s
}
