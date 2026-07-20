package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	headerToken          = "X-Plex-Token"
	headerContainerStart = "X-Plex-Container-Start"
	headerContainerSize  = "X-Plex-Container-Size"
)

// Client talks to a single Plex Media Server.
type Client struct {
	baseURL string // always ends with "/"
	token   string
	hc      *http.Client
}

// New builds a Client. baseURL must already be normalised to end with "/".
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
}

// get issues a GET against baseURL+path and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request for %q: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(headerToken, c.token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %q: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("plex API %q returned %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response from %q: %w", path, err)
	}
	return nil
}

// Libraries returns every library section on the server.
func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	var r containerResponse
	if err := c.get(ctx, "library/sections", nil, &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Directory, nil
}

// LibraryItems returns one page of items from a library. Plex pages via request
// headers rather than query parameters.
func (c *Client) LibraryItems(ctx context.Context, libraryID string, start, size int) ([]Metadata, error) {
	headers := map[string]string{
		headerContainerStart: strconv.Itoa(start),
		headerContainerSize:  strconv.Itoa(size),
	}
	var r containerResponse
	if err := c.get(ctx, "library/sections/"+libraryID+"/all", headers, &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}

// Metadata returns the detailed metadata (including file locations) for an item.
func (c *Client) Metadata(ctx context.Context, ratingKey string) ([]Metadata, error) {
	var r containerResponse
	if err := c.get(ctx, "library/metadata/"+ratingKey, nil, &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}

// Children returns the children of an item: the seasons of a show, or the
// episodes of a season.
func (c *Client) Children(ctx context.Context, ratingKey string) ([]Metadata, error) {
	var r containerResponse
	if err := c.get(ctx, "library/metadata/"+ratingKey+"/children", nil, &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}
