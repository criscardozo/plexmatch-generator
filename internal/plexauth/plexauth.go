// Package plexauth obtains a Plex authentication token via the plex.tv PIN
// (device-link) OAuth flow, validates it, and discovers the account's servers.
// It talks to plex.tv, which is a different host from the media server itself.
package plexauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// product identifies this client to Plex; it shows up in the account's
	// "authorised devices" list.
	product = "plexmatch-generator"

	defaultBaseURL      = "https://plex.tv/api/v2/"
	defaultPollInterval = 2 * time.Second
	defaultPollTimeout  = 5 * time.Minute
)

// NewClientID returns a random identifier for this device. It must be persisted
// so the same Pi is recognised as the same device across runs.
func NewClientID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating client identifier: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Client performs the plex.tv authentication and discovery calls.
type Client struct {
	hc           *http.Client
	baseURL      string
	clientID     string
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// NewClient builds a Client for the given persisted client identifier.
func NewClient(clientID string) *Client {
	return &Client{
		hc:           &http.Client{Timeout: 30 * time.Second},
		baseURL:      defaultBaseURL,
		clientID:     clientID,
		pollInterval: defaultPollInterval,
		pollTimeout:  defaultPollTimeout,
	}
}

// PIN is a device-link PIN returned by plex.tv.
type PIN struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
}

// CreatePIN requests a fresh PIN to start the device-link flow.
func (c *Client) CreatePIN(ctx context.Context) (PIN, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"pins?strong=true", nil)
	if err != nil {
		return PIN{}, fmt.Errorf("building pin request: %w", err)
	}
	c.setHeaders(req, "")

	var p PIN
	if err := c.do(req, &p); err != nil {
		return PIN{}, err
	}
	return p, nil
}

// AuthURL is the URL the user opens in a browser to authorise the PIN.
func (c *Client) AuthURL(p PIN) string {
	v := url.Values{}
	v.Set("clientID", c.clientID)
	v.Set("code", p.Code)
	v.Set("context[device][product]", product)
	return "https://app.plex.tv/auth#?" + v.Encode()
}

// PollPIN polls plex.tv until the PIN is authorised (returning the token), the
// context is cancelled, or the poll timeout elapses.
func (c *Client) PollPIN(ctx context.Context, p PIN) (string, error) {
	deadline := time.NewTimer(c.pollTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		token, err := c.checkPIN(ctx, p)
		if err != nil {
			return "", err
		}
		if token != "" {
			return token, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", errors.New("timed out waiting for authorisation")
		case <-ticker.C:
		}
	}
}

func (c *Client) checkPIN(ctx context.Context, p PIN) (string, error) {
	target := fmt.Sprintf("%spins/%d?code=%s", c.baseURL, p.ID, url.QueryEscape(p.Code))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("building pin poll request: %w", err)
	}
	c.setHeaders(req, "")

	var got PIN
	if err := c.do(req, &got); err != nil {
		return "", err
	}
	return got.AuthToken, nil
}

// ValidateToken reports whether a token is still accepted by plex.tv.
func (c *Client) ValidateToken(ctx context.Context, token string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"user", nil)
	if err != nil {
		return false, fmt.Errorf("building user request: %w", err)
	}
	c.setHeaders(req, token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("validating token: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusUnauthorized:
		return false, nil
	default:
		return false, fmt.Errorf("plex.tv /user returned %s", resp.Status)
	}
}

// Server is a Plex Media Server discovered for the account, with the best base
// URL to reach it.
type Server struct {
	Name    string
	BaseURL string
}

type resource struct {
	Name        string       `json:"name"`
	Provides    string       `json:"provides"`
	Connections []connection `json:"connections"`
}

type connection struct {
	URI   string `json:"uri"`
	Local bool   `json:"local"`
	Relay bool   `json:"relay"`
}

// DiscoverServers lists the account's media servers and their best base URL.
func (c *Client) DiscoverServers(ctx context.Context, token string) ([]Server, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"resources?includeHttps=1&includeRelay=1", nil)
	if err != nil {
		return nil, fmt.Errorf("building resources request: %w", err)
	}
	c.setHeaders(req, token)

	var resources []resource
	if err := c.do(req, &resources); err != nil {
		return nil, err
	}

	servers := []Server{}
	for _, res := range resources {
		if !strings.Contains(res.Provides, "server") {
			continue
		}
		if uri := bestConnection(res.Connections); uri != "" {
			servers = append(servers, Server{Name: res.Name, BaseURL: uri})
		}
	}
	return servers, nil
}

// bestConnection prefers a local, non-relay connection (ideal for a LAN device
// like a Raspberry Pi), then any non-relay connection, then a relay.
func bestConnection(conns []connection) string {
	var nonRelay, relay string
	for _, conn := range conns {
		switch {
		case conn.Relay:
			if relay == "" {
				relay = conn.URI
			}
		case conn.Local:
			return conn.URI
		default:
			if nonRelay == "" {
				nonRelay = conn.URI
			}
		}
	}
	if nonRelay != "" {
		return nonRelay
	}
	return relay
}

func (c *Client) setHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Product", product)
	req.Header.Set("X-Plex-Client-Identifier", c.clientID)
	if token != "" {
		req.Header.Set("X-Plex-Token", token)
	}
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %q: %w", req.URL.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("plex.tv %q returned %s: %s", req.URL.Path, resp.Status, strings.TrimSpace(string(body)))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response from %q: %w", req.URL.Path, err)
	}
	return nil
}
