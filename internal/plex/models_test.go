package plex

import (
	"encoding/json"
	"testing"
)

// Plex returns both a "guid" string (the primary GUID) and a "Guid" array of
// alternate IDs. This guards against the array clobbering the string under
// encoding/json's case-insensitive field matching.
func TestMetadataDecodingKeepsPrimaryGuid(t *testing.T) {
	t.Parallel()

	const body = `{"MediaContainer":{"Metadata":[{
		"ratingKey":"1727","title":"Heat","year":1995,
		"guid":"plex://movie/5d776b59",
		"Guid":[{"id":"imdb://tt0113277"},{"id":"tmdb://949"}]
	}]}}`

	var r containerResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	got := r.MediaContainer.Metadata[0]
	if got.Guid != "plex://movie/5d776b59" {
		t.Errorf("Guid = %q, want the primary plex:// guid", got.Guid)
	}
}

func TestMetadataIsNonDefaultOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ordering string
		want     bool
	}{
		{name: "empty is default", ordering: "", want: false},
		{name: "aired is non-default", ordering: "aired", want: true},
		{name: "absolute is non-default", ordering: "absolute", want: true},
		{name: "dvd is non-default", ordering: "dvd", want: true},
		{name: "tmdb is non-default", ordering: "tmdb", want: true},
		{name: "unknown value is default", ordering: "whatever", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := Metadata{ShowOrdering: tt.ordering}
			if got := m.IsNonDefaultOrdering(); got != tt.want {
				t.Errorf("IsNonDefaultOrdering() with %q = %v, want %v", tt.ordering, got, tt.want)
			}
		})
	}
}
