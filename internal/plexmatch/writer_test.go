package plexmatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "movie or show root",
			info: Info{Title: "Firefly", Year: 2002, Guid: "plex://show/5d9c081b"},
			want: "Title: Firefly\nYear: 2002\nGuid: plex://show/5d9c081b\n",
		},
		{
			name: "season",
			info: Info{Title: "Firefly", Year: 2002, Guid: "plex://season/602e67d4", Season: 1, IsSeason: true},
			want: "Title: Firefly\nYear: 2002\nSeason: 1\nGuid: plex://season/602e67d4\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Render(tt.info); got != tt.want {
				t.Errorf("Render() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), FileName)
	info := Info{Title: "Alias", Year: 2001, Guid: "plex://show/abc"}

	if err := Write(path, info); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != Render(info) {
		t.Errorf("file contents = %q, want %q", got, Render(info))
	}
}
