package plexauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBestConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		conns []connection
		want  string
	}{
		{
			name: "prefers local non-relay",
			conns: []connection{
				{URI: "https://relay", Relay: true},
				{URI: "https://remote"},
				{URI: "https://local", Local: true},
			},
			want: "https://local",
		},
		{
			name: "falls back to non-relay when no local",
			conns: []connection{
				{URI: "https://relay", Relay: true},
				{URI: "https://remote"},
			},
			want: "https://remote",
		},
		{
			name: "uses relay as a last resort",
			conns: []connection{
				{URI: "https://relay", Relay: true},
			},
			want: "https://relay",
		},
		{
			name:  "empty when no connections",
			conns: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := bestConnection(tt.conns); got != tt.want {
				t.Errorf("bestConnection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientAuthURL(t *testing.T) {
	t.Parallel()

	url := NewClient("client-123").AuthURL(PIN{Code: "abcXYZ"})
	for _, want := range []string{
		"https://app.plex.tv/auth#?",
		"clientID=client-123",
		"code=abcXYZ",
		"context%5Bdevice%5D%5Bproduct%5D=plexmatch-generator",
	} {
		if !strings.Contains(url, want) {
			t.Errorf("AuthURL() = %q, missing %q", url, want)
		}
	}
}

func TestClientPollPINSucceedsAfterAuthorisation(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The PIN is only claimed on the second poll.
		if calls.Add(1) >= 2 {
			_, _ = w.Write([]byte(`{"id":1,"code":"c","authToken":"the-token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"code":"c","authToken":null}`))
	}))
	defer srv.Close()

	c := NewClient("cid")
	c.baseURL = srv.URL + "/"
	c.pollInterval = time.Millisecond

	token, err := c.PollPIN(context.Background(), PIN{ID: 1, Code: "c"})
	if err != nil {
		t.Fatalf("PollPIN() error = %v", err)
	}
	if token != "the-token" {
		t.Errorf("PollPIN() = %q, want %q", token, "the-token")
	}
}

func TestClientValidateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "200 is valid", status: http.StatusOK, want: true},
		{name: "401 is invalid", status: http.StatusUnauthorized, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			c := NewClient("cid")
			c.baseURL = srv.URL + "/"

			got, err := c.ValidateToken(context.Background(), "t")
			if err != nil {
				t.Fatalf("ValidateToken() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ValidateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientDiscoverServers(t *testing.T) {
	t.Parallel()

	body := `[
		{"name":"Living Room","provides":"server","connections":[
			{"uri":"https://remote:32400","local":false,"relay":false},
			{"uri":"https://local:32400","local":true,"relay":false}
		]},
		{"name":"A Player","provides":"player","connections":[]}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Plex-Token"); got != "acct-token" {
			t.Errorf("X-Plex-Token = %q, want %q", got, "acct-token")
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient("cid")
	c.baseURL = srv.URL + "/"

	servers, err := c.DiscoverServers(context.Background(), "acct-token")
	if err != nil {
		t.Fatalf("DiscoverServers() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("DiscoverServers() returned %d servers, want 1 (players filtered out)", len(servers))
	}
	if servers[0].Name != "Living Room" || servers[0].BaseURL != "https://local:32400" {
		t.Errorf("DiscoverServers()[0] = %+v", servers[0])
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	// Redirect the config dir to a temp location. Setenv forbids t.Parallel().
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	if got, err := LoadCredentials(); err != nil || got != (Credentials{}) {
		t.Fatalf("LoadCredentials() on empty = %+v, %v; want zero value, nil", got, err)
	}

	want := Credentials{ClientID: "cid-1", Token: "tok-1"}
	if err := SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if got != want {
		t.Errorf("LoadCredentials() = %+v, want %+v", got, want)
	}

	if err := ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials() error = %v", err)
	}
	if got, _ := LoadCredentials(); got != (Credentials{}) {
		t.Errorf("after ClearCredentials(), LoadCredentials() = %+v, want zero value", got)
	}
}
