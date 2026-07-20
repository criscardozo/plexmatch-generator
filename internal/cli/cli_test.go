package cli

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, o *Options)
	}{
		{
			name: "short and long flags mix",
			args: []string{"-u", "http://192.168.0.3:32400", "--token", "ABC123"},
			check: func(t *testing.T, o *Options) {
				if o.URL != "http://192.168.0.3:32400" {
					t.Errorf("URL = %q", o.URL)
				}
				if o.Token != "ABC123" {
					t.Errorf("Token = %q", o.Token)
				}
				if o.ItemsPerPage != 20 {
					t.Errorf("ItemsPerPage = %d, want default 20", o.ItemsPerPage)
				}
			},
		},
		{
			name: "repeatable flags and root mapping",
			args: []string{"-t", "x", "-u", "http://x/", "-lib", "TV", "-lib", "Movies", "-r", "/mnt/media:/media"},
			check: func(t *testing.T, o *Options) {
				if len(o.LibraryNames) != 2 {
					t.Fatalf("LibraryNames = %v", o.LibraryNames)
				}
				if len(o.RootPaths) != 1 ||
					o.RootPaths[0].HostRootPath != "/mnt/media" ||
					o.RootPaths[0].PlexRootPath != "/media" {
					t.Errorf("RootPaths = %+v", o.RootPaths)
				}
			},
		},
		{
			name: "log path gets trailing slash",
			args: []string{"-t", "x", "-u", "http://x/", "-l", "/var/log"},
			check: func(t *testing.T, o *Options) {
				if o.LogPath != "/var/log/" {
					t.Errorf("LogPath = %q, want trailing slash", o.LogPath)
				}
			},
		},
		{
			name: "zero page size falls back to default",
			args: []string{"-t", "x", "-u", "http://x/", "-ps", "0"},
			check: func(t *testing.T, o *Options) {
				if o.ItemsPerPage != 20 {
					t.Errorf("ItemsPerPage = %d, want 20", o.ItemsPerPage)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			tt.check(t, o)
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()
	if _, err := Parse([]string{"--version"}); !errors.Is(err, ErrVersion) {
		t.Errorf("Parse(--version) error = %v, want ErrVersion", err)
	}
}

func TestParseRootPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []RootPath
	}{
		{
			name: "valid mapping",
			in:   []string{"/mnt/media:/media"},
			want: []RootPath{{HostRootPath: "/mnt/media", PlexRootPath: "/media"}},
		},
		{
			name: "missing separator is ignored",
			in:   []string{"just-one-path"},
			want: nil,
		},
		{
			name: "empty side is ignored",
			in:   []string{":/media", "/host:"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseRootPaths(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("parseRootPaths() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
