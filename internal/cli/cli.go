// Package cli parses command-line arguments into a validated Options value.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// ErrVersion is returned by Parse when the user requested --version. The caller
// is expected to print the build version and exit cleanly.
var ErrVersion = errors.New("version requested")

// RootPath maps a path as Plex reports it to the equivalent path on the host
// running this tool. This matters when Plex runs in a container that mounts the
// media somewhere different (e.g. Plex sees /media, the host sees /mnt/media).
type RootPath struct {
	HostRootPath string
	PlexRootPath string
}

// Options holds every setting the generator needs for a run.
type Options struct {
	Token            string
	URL              string
	RootPaths        []RootPath
	LogPath          string
	NoOverwrite      bool
	ItemsPerPage     int
	LibraryNames     []string
	ShowNames        []string
	SeasonProcessing bool
	Relogin          bool
	ServerName       string
	Logout           bool
}

// stringSlice collects a repeatable string flag (e.g. -lib TV -lib Movies).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// Parse turns raw args into Options. It returns flag.ErrHelp when -h/--help is
// requested and ErrVersion when --version is requested.
func Parse(args []string) (*Options, error) {
	fs := flag.NewFlagSet("plexmatch-generator", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {} // help text is printed by us on flag.ErrHelp

	var (
		token, url, logPath, serverName            string
		pageSize                                   int
		noOverwrite, seasonProcessing, showVersion bool
		relogin, logout                            bool
		roots, libraries, shows                    stringSlice
	)

	// Each option has a long form and a short form; both write to the same
	// variable so whichever the user passes wins.
	fs.StringVar(&token, "token", "", "Plex server token (required)")
	fs.StringVar(&token, "t", "", "shorthand for --token")
	fs.StringVar(&url, "url", "", "Plex server URL, e.g. http://192.168.0.3:32400 (required)")
	fs.StringVar(&url, "u", "", "shorthand for --url")
	fs.Var(&roots, "root", "root path mapping hostPath:plexPath (repeatable)")
	fs.Var(&roots, "r", "shorthand for --root")
	fs.StringVar(&logPath, "log", "", "directory in which to write plexmatch.log (must already exist)")
	fs.StringVar(&logPath, "l", "", "shorthand for --log")
	fs.IntVar(&pageSize, "pagesize", 20, "batch size used when paging through a library")
	fs.IntVar(&pageSize, "ps", 20, "shorthand for --pagesize")
	fs.BoolVar(&noOverwrite, "nooverwrite", false, "skip folders that already contain a .plexmatch file")
	fs.BoolVar(&noOverwrite, "no", false, "shorthand for --nooverwrite")
	fs.Var(&libraries, "library", "restrict to a library by name (repeatable, case-insensitive)")
	fs.Var(&libraries, "lib", "shorthand for --library")
	fs.Var(&shows, "show", "restrict to a media item by title (repeatable, case-insensitive)")
	fs.Var(&shows, "s", "shorthand for --show")
	fs.BoolVar(&seasonProcessing, "seasonprocessing", false, "force writing a .plexmatch in every season folder too")
	fs.BoolVar(&seasonProcessing, "sp", false, "shorthand for --seasonprocessing")
	fs.BoolVar(&relogin, "relogin", false, "ignore any cached token and authenticate with Plex again")
	fs.StringVar(&serverName, "server-name", "", "select a discovered server by name (used when --url is omitted)")
	fs.BoolVar(&logout, "logout", false, "delete the cached Plex token and exit")
	fs.BoolVar(&showVersion, "version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(helpText)
			return nil, flag.ErrHelp
		}
		return nil, err
	}

	if showVersion {
		return nil, ErrVersion
	}

	// A page size of 0 is meaningless; fall back to the default, matching the
	// behaviour of the original tool.
	if pageSize <= 0 {
		pageSize = 20
	}

	// Serilog's file sink expected the log path to end in a separator; keep the
	// same convention so the log file lands inside the directory given.
	if logPath != "" && !strings.HasSuffix(logPath, "/") {
		logPath += "/"
	}

	return &Options{
		Token:            token,
		URL:              url,
		RootPaths:        parseRootPaths(roots),
		LogPath:          logPath,
		NoOverwrite:      noOverwrite,
		ItemsPerPage:     pageSize,
		LibraryNames:     libraries,
		ShowNames:        shows,
		SeasonProcessing: seasonProcessing,
		Relogin:          relogin,
		ServerName:       serverName,
		Logout:           logout,
	}, nil
}

// parseRootPaths turns "hostPath:plexPath" strings into RootPath values. Entries
// that don't contain two non-empty parts are ignored, as in the original tool.
func parseRootPaths(raw []string) []RootPath {
	var out []RootPath
	for _, m := range raw {
		parts := strings.SplitN(m, ":", 2)
		if len(parts) != 2 {
			continue
		}
		host := strings.TrimSpace(parts[0])
		plex := strings.TrimSpace(parts[1])
		if host == "" || plex == "" {
			continue
		}
		out = append(out, RootPath{HostRootPath: host, PlexRootPath: plex})
	}
	return out
}

const helpText = `plexmatch-generator - write .plexmatch hint files for a Plex Media Server

Usage:
  plexmatch-generator [options]

On the first run, if no token is given, the tool prints a URL to authorise this
device with your Plex account, then caches the token for next time. If no --url
is given, it discovers your server automatically.

Authentication:
  -t,   --token <token>   Plex authentication token (X-Plex-Token). Optional;
                          overrides the cached token when given.
        --relogin         Ignore the cached token and authenticate again.
        --logout          Delete the cached token and exit.

Server:
  -u,   --url <url>       Plex server URL (http:// or https://). Optional; when
                          omitted the server is discovered from your account.
        --server-name <n> Pick a discovered server by name (when --url is omitted
                          and the account has more than one server).

Options:
  -r,   --root <map>      Map a Plex path to a host path, "hostPath:plexPath".
                          Repeatable. Example: -r /mnt/media:/media
  -lib, --library <name>  Only process this library (repeatable, case-insensitive).
  -s,   --show <title>    Only process this media item (repeatable, case-insensitive).
  -sp,  --seasonprocessing
                          Also write a .plexmatch in every season folder. This is
                          already automatic for shows using non-default ordering.
  -no,  --nooverwrite     Do not overwrite folders that already have a .plexmatch.
  -ps,  --pagesize <n>    Items requested per page when scanning a library (default 20).
  -l,   --log <dir>       Also append logs to <dir>/plexmatch.log (dir must exist).
        --version         Print the version and exit.
  -h,   --help            Show this help.
`
