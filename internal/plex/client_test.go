package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
