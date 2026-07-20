package plex

import "testing"

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
