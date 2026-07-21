package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientRetriesTransientServerError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "busy", http.StatusServiceUnavailable) // transient 503
			return
		}
		_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie","title":"Movies"}]}}`))
	}))
	defer srv.Close()

	c := New(srv.URL+"/", "t")
	c.retryWait = 0 // no delay in the test

	libs, err := c.Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries() error = %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("got %d libraries after retry, want 1", len(libs))
	}
	if calls.Load() != 2 {
		t.Errorf("server called %d times, want 2 (1 failure + 1 retry)", calls.Load())
	}
}

func TestClientDoesNotRetryClientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", http.StatusUnauthorized) // 401 is not retryable
	}))
	defer srv.Close()

	c := New(srv.URL+"/", "t")
	c.retryWait = 0

	if _, err := c.Libraries(context.Background()); err == nil {
		t.Fatal("Libraries() error = nil, want an error on HTTP 401")
	}
	if calls.Load() != 1 {
		t.Errorf("server called %d times, want 1 (no retry on 4xx)", calls.Load())
	}
}

func TestClientLibraries(t *testing.T) {
	t.Parallel()

	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get(headerToken)
		_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie","title":"Movies"}]}}`))
	}))
	defer srv.Close()

	libs, err := New(srv.URL+"/", "secret-token").Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries() error = %v", err)
	}
	if gotToken != "secret-token" {
		t.Errorf("X-Plex-Token header = %q, want %q", gotToken, "secret-token")
	}
	if len(libs) != 1 || libs[0].ID != "1" || libs[0].Type != "movie" || libs[0].Title != "Movies" {
		t.Errorf("Libraries() = %+v", libs)
	}
}

func TestClientLibraryItemsSendsPagingHeaders(t *testing.T) {
	t.Parallel()

	var gotStart, gotSize string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStart = r.Header.Get(headerContainerStart)
		gotSize = r.Header.Get(headerContainerSize)
		_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"42","title":"Heat","year":1995}]}}`))
	}))
	defer srv.Close()

	items, err := New(srv.URL+"/", "t").LibraryItems(context.Background(), "1", 40, 20)
	if err != nil {
		t.Fatalf("LibraryItems() error = %v", err)
	}
	if gotStart != "40" || gotSize != "20" {
		t.Errorf("paging headers start=%q size=%q, want 40/20", gotStart, gotSize)
	}
	if len(items) != 1 || items[0].RatingKey != "42" || items[0].Year != 1995 {
		t.Errorf("LibraryItems() = %+v", items)
	}
}

func TestClientReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	if _, err := New(srv.URL+"/", "t").Libraries(context.Background()); err == nil {
		t.Fatal("Libraries() error = nil, want an error on HTTP 401")
	}
}
