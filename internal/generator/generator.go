// Package generator drives the whole run: it talks to Plex, decides where each
// .plexmatch file belongs, and writes it.
package generator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"plexmatch-generator/internal/cli"
	"plexmatch-generator/internal/plex"
	"plexmatch-generator/internal/plexmatch"
)

// Run executes a full generation pass and returns the process exit code.
func Run(ctx context.Context, opts *cli.Options) int {
	logger, closeLog, err := newLogger(opts.LogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open log file: %v\n", err)
		return 1
	}
	defer closeLog()

	// Prompt for anything not supplied on the command line.
	if opts.Token == "" {
		fmt.Println("Please enter your Plex token:")
		opts.Token = readLine()
	}
	if strings.TrimSpace(opts.Token) == "" {
		logger.Error("The provided Plex token is invalid.")
		return 1
	}

	if opts.URL == "" {
		fmt.Println("Please enter your Plex server URL:")
		opts.URL = readLine()
	}
	baseURL, ok := normaliseURL(opts.URL)
	if !ok {
		logger.Error("The provided Plex URL is invalid; it must start with http:// or https://")
		return 1
	}

	r := &runner{
		client: plex.New(baseURL, opts.Token),
		log:    logger,
		opts:   opts,
	}

	// A single top-level error handler mirrors the original's global try/catch:
	// any API or write failure aborts the run with exit code 1.
	if err := r.run(ctx); err != nil {
		logger.Error("A fatal error occurred", "error", err)
		return 1
	}

	logger.Info("Processing complete.")
	return 0
}

type runner struct {
	client *plex.Client
	log    *slog.Logger
	opts   *cli.Options
}

// processingResult accumulates counts for one library.
type processingResult struct {
	processed int
	skipped   int
	success   bool
}

func (r *runner) run(ctx context.Context) error {
	libraries, err := r.client.Libraries(ctx)
	if err != nil {
		return err
	}
	if len(libraries) == 0 {
		return errors.New("no libraries were returned from the Plex server")
	}

	for _, library := range libraries {
		if len(r.opts.LibraryNames) > 0 && !containsFold(r.opts.LibraryNames, library.Title) {
			r.log.Info("Skipping library (not in allow list)", "library", library.Title)
			continue
		}

		res, err := r.processLibrary(ctx, library)
		if err != nil {
			return err
		}
		switch {
		case !res.success:
			r.log.Error("No results for library", "library", library.Title, "type", library.Type, "id", library.ID)
		case res.skipped > 0:
			r.log.Info("Library processed", "library", library.Title, "processed", res.processed, "skipped", res.skipped)
		default:
			r.log.Info("Library processed", "library", library.Title, "processed", res.processed)
		}
	}
	return nil
}

// processLibrary pages through a library and processes every item.
func (r *runner) processLibrary(ctx context.Context, library plex.Library) (processingResult, error) {
	res := processingResult{success: true}

	for start := 0; ; start += r.opts.ItemsPerPage {
		items, err := r.client.LibraryItems(ctx, library.ID, start, r.opts.ItemsPerPage)
		if err != nil {
			return res, err
		}
		if len(items) == 0 {
			// No items at all means the library came back empty.
			if start == 0 {
				res.success = false
			}
			break
		}

		for _, item := range items {
			if len(r.opts.ShowNames) > 0 && !containsFold(r.opts.ShowNames, item.Title) {
				r.log.Info("Skipping item (not in allow list)", "title", item.Title)
				res.skipped++
				continue
			}
			res.processed++

			if err := r.processItem(ctx, library, item); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// processItem writes the top-level .plexmatch for an item and, for shows, may
// also write per-season files.
func (r *runner) processItem(ctx context.Context, library plex.Library, item plex.Metadata) error {
	infos, err := r.client.Metadata(ctx, item.RatingKey)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		r.log.Error("No location info found for item", "title", item.Title, "id", item.RatingKey)
		return nil
	}

	for _, info := range infos {
		paths, ok := mediaFolders(library, info)
		if !ok {
			r.log.Warn("No media found for item", "title", item.Title)
			continue
		}

		for _, folder := range paths {
			folder = r.mapRootPath(folder)
			if err := r.writeMatch(folder, plexmatch.Info{
				Title: item.Title,
				Year:  item.Year,
				Guid:  item.Guid,
			}); err != nil {
				return err
			}
		}

		// Per-season processing runs when the show uses non-default episode
		// ordering, or when the user forced it with --seasonprocessing.
		isShow := item.Type == "show"
		seasonProcessingWanted := r.opts.SeasonProcessing || info.IsNonDefaultOrdering()
		if isShow && seasonProcessingWanted {
			if err := r.processSeasons(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// processSeasons writes a .plexmatch into each season folder of a show, using
// the season's own GUID and index.
func (r *runner) processSeasons(ctx context.Context, item plex.Metadata) error {
	seasons, err := r.client.Children(ctx, item.RatingKey)
	if err != nil {
		return err
	}
	if len(seasons) == 0 {
		return nil
	}

	// Deduplicate by folder across all seasons; the first season to claim a
	// folder wins, matching the original tool.
	seen := make(map[string]plexmatch.Info)
	var order []string

	for _, season := range seasons {
		episodes, err := r.client.Children(ctx, season.RatingKey)
		if err != nil {
			return err
		}
		if len(episodes) == 0 {
			continue
		}

		for _, episode := range episodes {
			for _, media := range episode.Media {
				for _, part := range media.Part {
					folder := parentDir(part.File)
					if _, exists := seen[folder]; exists {
						continue
					}
					seen[folder] = plexmatch.Info{
						Title:    item.Title,
						Year:     item.Year,
						Guid:     season.Guid, // season GUID, not the show's
						Season:   season.Index,
						IsSeason: true,
					}
					order = append(order, folder)
				}
			}
		}
	}

	for _, folder := range order {
		if err := r.writeMatch(r.mapRootPath(folder), seen[folder]); err != nil {
			return err
		}
	}
	return nil
}

// writeMatch writes a .plexmatch into folder, honouring --nooverwrite and
// skipping (with a log line) folders that do not exist on this host.
func (r *runner) writeMatch(folder string, info plexmatch.Info) error {
	if !dirExists(folder) {
		r.log.Error("Folder is missing or invalid", "path", folder)
		return nil
	}

	target := filepath.Join(folder, plexmatch.FileName)
	if r.opts.NoOverwrite && fileExists(target) {
		r.log.Info("Skipping existing .plexmatch (overwrite disabled)", "title", info.Title)
		return nil
	}

	if err := plexmatch.Write(target, info); err != nil {
		return fmt.Errorf("writing %q: %w", target, err)
	}

	if info.IsSeason {
		r.log.Info(".plexmatch (season) written", "title", info.Title, "season", info.Season, "path", folder)
	} else {
		r.log.Info(".plexmatch written", "title", info.Title)
	}
	return nil
}

// mapRootPath rewrites a Plex-reported path to the equivalent host path using
// the configured root mappings. The first matching prefix wins.
func (r *runner) mapRootPath(path string) string {
	for _, rp := range r.opts.RootPaths {
		if strings.HasPrefix(path, rp.PlexRootPath) {
			return rp.HostRootPath + strings.TrimPrefix(path, rp.PlexRootPath)
		}
	}
	return path
}

// mediaFolders returns the candidate folders for an item based on library type.
// Movies expose file paths (we keep the containing folder); shows and music
// expose folder paths directly.
func mediaFolders(library plex.Library, info plex.Metadata) ([]string, bool) {
	switch {
	case library.Type == "movie" && info.Media != nil:
		var out []string
		for _, media := range info.Media {
			for _, part := range media.Part {
				out = append(out, parentDir(part.File))
			}
		}
		return out, true
	case (library.Type == "show" || library.Type == "artist") && info.Location != nil:
		var out []string
		for _, loc := range info.Location {
			out = append(out, loc.Path)
		}
		return out, true
	default:
		return nil, false
	}
}

// parentDir returns everything up to (but not including) the last path
// separator, handling both "/" and "\" so paths from any Plex host work.
func parentDir(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return p
	}
	return p[:i]
}

func containsFold(list []string, value string) bool {
	for _, s := range list {
		if strings.EqualFold(s, value) {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// normaliseURL ensures a trailing slash and validates the scheme.
func normaliseURL(u string) (string, bool) {
	u = strings.TrimSpace(u)
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u, true
	}
	return "", false
}

func readLine() string {
	// ReadString returns whatever it read even on error (e.g. io.EOF when stdin
	// is closed), and the downstream token/URL validation rejects empty input,
	// so a read error here needs no separate handling.
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}

// newLogger writes to stdout and, when logPath is set, also appends to
// <logPath>plexmatch.log.
func newLogger(logPath string) (*slog.Logger, func(), error) {
	writers := []io.Writer{os.Stdout}
	closer := func() {}

	if logPath != "" {
		f, err := os.OpenFile(filepath.Join(logPath, "plexmatch.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, f)
		closer = func() { _ = f.Close() }
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), closer, nil
}
