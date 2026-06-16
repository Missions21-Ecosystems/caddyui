package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps the Caddy admin REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Caddy API client.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Ping returns true if the Caddy admin API responds.
func (c *Client) Ping() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/config/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// GetConfig fetches the Caddy config at the given path (empty = root).
func (c *Client) GetConfig(path string) (json.RawMessage, error) {
	url := c.baseURL + "/config/"
	if path != "" {
		url += strings.TrimLeft(path, "/")
	}
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, body)
	}
	return json.RawMessage(body), nil
}

// LoadConfig replaces the entire Caddy config via POST /load.
// It always injects admin.listen = 0.0.0.0:2019 so the API stays reachable
// across Docker networks after the config swap.
func (c *Client) LoadConfig(config json.RawMessage) error {
	patched, err := ensureAdminListen(config)
	if err != nil {
		patched = config // fall back to unmodified
	}
	return c.doJSON(http.MethodPost, c.baseURL+"/load", patched)
}

// ensureAdminListen injects {"admin":{"listen":"0.0.0.0:2019"}} into the
// config if the admin listen address is absent or bound only to localhost.
func ensureAdminListen(config json.RawMessage) (json.RawMessage, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, err
	}
	var admin AdminConfig
	if raw, ok := cfg["admin"]; ok {
		json.Unmarshal(raw, &admin)
	}
	if admin.Listen == "" || admin.Listen == "localhost:2019" || admin.Listen == "127.0.0.1:2019" {
		admin.Listen = "0.0.0.0:2019"
		b, err := json.Marshal(admin)
		if err != nil {
			return nil, err
		}
		cfg["admin"] = json.RawMessage(b)
	}
	return json.Marshal(cfg)
}

// SetConfig sets the config at the given path via POST /config/[path].
func (c *Client) SetConfig(path string, body json.RawMessage) error {
	url := c.baseURL + "/config/" + strings.TrimLeft(path, "/")
	return c.doJSON(http.MethodPost, url, body)
}

// DeleteConfig removes the config value at the given path.
func (c *Client) DeleteConfig(path string) error {
	url := c.baseURL + "/config/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

// StopServer gracefully stops the Caddy server.
func (c *Client) StopServer() error {
	resp, err := c.httpClient.Post(c.baseURL+"/stop", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

// AdaptConfig converts a Caddyfile to Caddy JSON via POST /adapt.
// Caddy expects the raw config body with the adapter name as Content-Type.
func (c *Client) AdaptConfig(input, adapter string) (json.RawMessage, error) {
	contentType := "text/" + adapter
	resp, err := c.httpClient.Post(c.baseURL+"/adapt", contentType, bytes.NewReader([]byte(input)))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, body)
	}
	return json.RawMessage(body), nil
}

// GetUpstreams returns all reverse proxy upstream statuses.
func (c *Client) GetUpstreams() ([]Upstream, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/reverse_proxy/upstreams")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, body)
	}
	var upstreams []Upstream
	if err := json.Unmarshal(body, &upstreams); err != nil {
		return nil, fmt.Errorf("decode upstreams: %w", err)
	}
	return upstreams, nil
}

// GetPKICA fetches info for the given CA ID (e.g. "local").
func (c *Client) GetPKICA(id string) (*PKICAInfo, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/pki/ca/" + id)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, body)
	}
	var info PKICAInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode CA info: %w", err)
	}
	return &info, nil
}

// GetPKICACerts fetches the certificate chain for the given CA ID.
func (c *Client) GetPKICACerts(id string) (json.RawMessage, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/pki/ca/" + id + "/certificates")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy returned %d: %s", resp.StatusCode, body)
	}
	return json.RawMessage(body), nil
}

// doJSON sends a JSON body with the given method and handles the response.
func (c *Client) doJSON(method, url string, body json.RawMessage) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy returned %d: %s", resp.StatusCode, b)
	}
	return nil
}
