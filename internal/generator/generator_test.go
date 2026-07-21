package generator

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"plexmatch-generator/internal/cli"
	"plexmatch-generator/internal/plex"
	"plexmatch-generator/internal/ui"
)

// stubClient is an in-memory plexClient for exercising the generator without a
// real server.
type stubClient struct {
	libraries []plex.Library
	items     map[string][]plex.Metadata // libraryID -> single page of items
	metadata  map[string][]plex.Metadata // ratingKey -> location infos
	children  map[string][]plex.Metadata // ratingKey -> children
}

func (s *stubClient) Libraries(context.Context) ([]plex.Library, error) {
	return s.libraries, nil
}

func (s *stubClient) LibraryItems(_ context.Context, id string, start, _ int) ([]plex.Metadata, error) {
	if start > 0 {
		return nil, nil // single page, then empty
	}
	return s.items[id], nil
}

func (s *stubClient) Metadata(_ context.Context, key string) ([]plex.Metadata, error) {
	return s.metadata[key], nil
}

func (s *stubClient) Children(_ context.Context, key string) ([]plex.Metadata, error) {
	return s.children[key], nil
}

func discardReporter() *ui.Reporter { return ui.New(io.Discard, nil, false, false) }

func TestRunnerWritesMovieMatch(t *testing.T) {
	t.Parallel()

	movieDir := filepath.Join(t.TempDir(), "Heat (1995)")
	if err := os.Mkdir(movieDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stub := &stubClient{
		metadata: map[string][]plex.Metadata{
			"1": {{Media: []plex.Media{{Part: []plex.Part{{File: filepath.Join(movieDir, "heat.mkv")}}}}}},
		},
	}
	r := &runner{client: stub, rep: discardReporter(), opts: &cli.Options{ItemsPerPage: 20}}
	item := plex.Metadata{RatingKey: "1", Title: "Heat", Year: 1995, Guid: "plex://movie/x", Type: "movie"}

	if err := r.processItem(context.Background(), plex.Library{Type: "movie"}, item); err != nil {
		t.Fatalf("processItem error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(movieDir, ".plexmatch"))
	if err != nil {
		t.Fatalf("reading .plexmatch: %v", err)
	}
	if want := "Title: Heat\nYear: 1995\nGuid: plex://movie/x\n"; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestRunnerWritesSeasonMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	showDir := filepath.Join(root, "Firefly")
	seasonDir := filepath.Join(showDir, "Season 1")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stub := &stubClient{
		metadata: map[string][]plex.Metadata{
			"show1": {{Location: []plex.Location{{Path: showDir}}}},
		},
		children: map[string][]plex.Metadata{
			"show1":   {{RatingKey: "season1", Guid: "plex://season/s1", Index: 1}},
			"season1": {{Media: []plex.Media{{Part: []plex.Part{{File: filepath.Join(seasonDir, "e01.mkv")}}}}}},
		},
	}
	r := &runner{client: stub, rep: discardReporter(), opts: &cli.Options{ItemsPerPage: 20, SeasonProcessing: true}}
	item := plex.Metadata{RatingKey: "show1", Title: "Firefly", Year: 2002, Guid: "plex://show/x", Type: "show"}

	if err := r.processItem(context.Background(), plex.Library{Type: "show"}, item); err != nil {
		t.Fatalf("processItem error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(showDir, ".plexmatch")); err != nil {
		t.Errorf("show-level .plexmatch not written: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(seasonDir, ".plexmatch"))
	if err != nil {
		t.Fatalf("reading season .plexmatch: %v", err)
	}
	if want := "Title: Firefly\nYear: 2002\nSeason: 1\nGuid: plex://season/s1\n"; string(got) != want {
		t.Errorf("season content = %q, want %q", got, want)
	}
}

func TestRunnerDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	movieDir := filepath.Join(t.TempDir(), "Heat")
	if err := os.Mkdir(movieDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stub := &stubClient{
		metadata: map[string][]plex.Metadata{
			"1": {{Media: []plex.Media{{Part: []plex.Part{{File: filepath.Join(movieDir, "h.mkv")}}}}}},
		},
	}
	rep := ui.New(io.Discard, nil, false, true)
	r := &runner{client: stub, rep: rep, opts: &cli.Options{ItemsPerPage: 20, DryRun: true}}
	item := plex.Metadata{RatingKey: "1", Title: "Heat", Type: "movie"}

	if err := r.processItem(context.Background(), plex.Library{Type: "movie"}, item); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(movieDir, ".plexmatch")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not write a file (stat err = %v)", err)
	}
}

func TestRunnerAppliesRootMapping(t *testing.T) {
	t.Parallel()

	host := t.TempDir()
	movieHost := filepath.Join(host, "Heat")
	if err := os.Mkdir(movieHost, 0o755); err != nil {
		t.Fatal(err)
	}

	// Plex reports the file under /media; the host has it under the temp dir.
	stub := &stubClient{
		metadata: map[string][]plex.Metadata{
			"1": {{Media: []plex.Media{{Part: []plex.Part{{File: "/media/Heat/h.mkv"}}}}}},
		},
	}
	opts := &cli.Options{
		ItemsPerPage: 20,
		RootPaths:    []cli.RootPath{{HostRootPath: host, PlexRootPath: "/media"}},
	}
	r := &runner{client: stub, rep: discardReporter(), opts: opts}
	item := plex.Metadata{RatingKey: "1", Title: "Heat", Guid: "plex://movie/x", Type: "movie"}

	if err := r.processItem(context.Background(), plex.Library{Type: "movie"}, item); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(movieHost, ".plexmatch")); err != nil {
		t.Errorf("root-mapped write missing: %v", err)
	}
}

func TestRunnerRunProcessesLibrary(t *testing.T) {
	t.Parallel()

	movieDir := filepath.Join(t.TempDir(), "A")
	if err := os.Mkdir(movieDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stub := &stubClient{
		libraries: []plex.Library{{ID: "1", Type: "movie", Title: "Movies"}},
		items:     map[string][]plex.Metadata{"1": {{RatingKey: "10", Title: "A", Type: "movie", Guid: "plex://movie/a"}}},
		metadata:  map[string][]plex.Metadata{"10": {{Media: []plex.Media{{Part: []plex.Part{{File: filepath.Join(movieDir, "a.mkv")}}}}}}},
	}
	r := &runner{client: stub, rep: discardReporter(), opts: &cli.Options{ItemsPerPage: 20}}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(movieDir, ".plexmatch")); err != nil {
		t.Errorf(".plexmatch not written by run: %v", err)
	}
}

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
