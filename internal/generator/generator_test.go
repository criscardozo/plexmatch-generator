package generator

import (
	"slices"
	"testing"

	"plexmatch-generator/internal/cli"
	"plexmatch-generator/internal/plex"
)

func TestRunnerMapRootPath(t *testing.T) {
	t.Parallel()

	r := &runner{opts: &cli.Options{RootPaths: []cli.RootPath{
		{HostRootPath: "/mnt/media", PlexRootPath: "/media"},
	}}}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "maps matching prefix to host", in: "/media/Movies/Heat", want: "/mnt/media/Movies/Heat"},
		{name: "leaves non-matching path untouched", in: "/other/Movies/Heat", want: "/other/Movies/Heat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.mapRootPath(tt.in); got != tt.want {
				t.Errorf("mapRootPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMediaFolders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		library plex.Library
		info    plex.Metadata
		want    []string
		wantOK  bool
	}{
		{
			name:    "movie uses the file's parent folder",
			library: plex.Library{Type: "movie"},
			info:    plex.Metadata{Media: []plex.Media{{Part: []plex.Part{{File: "/media/Movies/Heat (1995)/heat.mkv"}}}}},
			want:    []string{"/media/Movies/Heat (1995)"},
			wantOK:  true,
		},
		{
			name:    "show uses the location path directly",
			library: plex.Library{Type: "show"},
			info:    plex.Metadata{Location: []plex.Location{{Path: "/media/TV/Firefly"}}},
			want:    []string{"/media/TV/Firefly"},
			wantOK:  true,
		},
		{
			name:    "unknown library type is not ok",
			library: plex.Library{Type: "photo"},
			info:    plex.Metadata{},
			want:    nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := mediaFolders(tt.library, tt.info)
			if ok != tt.wantOK {
				t.Fatalf("mediaFolders() ok = %v, want %v", ok, tt.wantOK)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("mediaFolders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParentDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "unix path", in: "/media/Movies/Firefly (2002)/movie.mkv", want: "/media/Movies/Firefly (2002)"},
		{name: "windows path", in: `C:\Media\Movies\movie.mkv`, want: `C:\Media\Movies`},
		{name: "no separator returns input", in: "movie.mkv", want: "movie.mkv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parentDir(tt.in); got != tt.want {
				t.Errorf("parentDir(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	t.Parallel()

	if !containsFold([]string{"TV Shows"}, "tv shows") {
		t.Error("containsFold should match case-insensitively")
	}
	if containsFold([]string{"Movies"}, "TV") {
		t.Error("containsFold should not match a different value")
	}
}

func TestNormaliseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		want      string
		wantValid bool
	}{
		{name: "adds trailing slash", in: "http://x:32400", want: "http://x:32400/", wantValid: true},
		{name: "keeps existing trailing slash", in: "https://x/", want: "https://x/", wantValid: true},
		{name: "rejects non-http scheme", in: "ftp://x", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := normaliseURL(tt.in)
			if ok != tt.wantValid {
				t.Fatalf("normaliseURL(%q) ok = %v, want %v", tt.in, ok, tt.wantValid)
			}
			if ok && got != tt.want {
				t.Errorf("normaliseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
