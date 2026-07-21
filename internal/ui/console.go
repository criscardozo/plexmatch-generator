// Package ui renders the human-facing console output: clean, coloured lines for
// a terminal, plus optional plain, timestamped lines to a log file.
package ui

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ANSI colour codes. They are emitted only when colour is enabled.
const (
	codeReset  = "\033[0m"
	codeBold   = "\033[1m"
	codeDim    = "\033[2m"
	codeRed    = "\033[31m"
	codeGreen  = "\033[32m"
	codeYellow = "\033[33m"
	codeCyan   = "\033[36m"
)

var separator = strings.Repeat("─", 42)

// Reporter prints progress to a terminal and, optionally, to a log file. It
// also tallies what happened for the final summary.
type Reporter struct {
	out    io.Writer
	file   io.Writer // optional; nil when no --log is set
	color  bool
	dryRun bool
	now    func() time.Time

	started time.Time
	written int
	skipped int
	issues  int
}

// New builds a Reporter writing to out (coloured when color is true) and, when
// file is non-nil, mirroring plain timestamped lines to it. When dryRun is
// true, "wrote" events are reported as "would write" and no summary implies a
// change on disk.
func New(out, file io.Writer, color, dryRun bool) *Reporter {
	return NewWithClock(out, file, color, dryRun, time.Now)
}

// NewWithClock is like New but takes an explicit clock, for tests.
func NewWithClock(out, file io.Writer, color, dryRun bool, now func() time.Time) *Reporter {
	return &Reporter{out: out, file: file, color: color, dryRun: dryRun, now: now, started: now()}
}

// Start prints the header naming the server being processed.
func (r *Reporter) Start(serverName, url string) {
	name := serverName
	if name == "" {
		name = url
	}
	fmt.Fprintf(r.out, "\n  %s %s\n", r.paint(codeDim, "Plex"), r.paint(codeBold+codeCyan, name))
	if serverName != "" {
		fmt.Fprintf(r.out, "  %s\n", r.paint(codeDim, url))
	}
	if r.dryRun {
		fmt.Fprintf(r.out, "  %s\n", r.paint(codeDim, "(dry run — no files are written)"))
	}
	fmt.Fprintf(r.out, "  %s\n\n", r.paint(codeDim, separator))
	r.logf("using server %q (%s)", serverName, url)
}

// Wrote records a top-level .plexmatch write (or, in dry-run, a would-write).
func (r *Reporter) Wrote(title string) {
	r.written++
	symbol, code, verb := r.writeStyle()
	fmt.Fprintf(r.out, "  %s %s\n", r.paint(code, symbol), title)
	r.logf("%s %q", verb, title)
}

// WroteSeason records a per-season .plexmatch write (or would-write in dry-run).
func (r *Reporter) WroteSeason(title string, season int) {
	r.written++
	symbol, code, verb := r.writeStyle()
	tail := r.paint(codeDim, fmt.Sprintf("· season %d", season))
	fmt.Fprintf(r.out, "  %s %s %s\n", r.paint(code, symbol), title, tail)
	r.logf("%s %q season %d", verb, title, season)
}

// writeStyle returns the symbol, colour and log verb for a write event,
// depending on whether this is a dry run.
func (r *Reporter) writeStyle() (symbol, code, verb string) {
	if r.dryRun {
		return "~", codeCyan, "would write"
	}
	return "✓", codeGreen, "wrote"
}

// Skip records a folder that was intentionally not written (filtered out, or
// already present under --nooverwrite). It produces no console line.
func (r *Reporter) Skip() {
	r.skipped++
}

// Warn reports a recoverable problem (e.g. an item with no media).
func (r *Reporter) Warn(msg string) {
	r.issues++
	fmt.Fprintf(r.out, "  %s %s\n", r.paint(codeYellow, "!"), msg)
	r.logf("warning: %s", msg)
}

// Fail reports a problem that stopped one item but not the whole run.
func (r *Reporter) Fail(msg string) {
	r.issues++
	fmt.Fprintf(r.out, "  %s %s\n", r.paint(codeRed, "✗"), msg)
	r.logf("error: %s", msg)
}

// Note prints a low-key informational line (dimmed).
func (r *Reporter) Note(msg string) {
	fmt.Fprintf(r.out, "  %s %s\n", r.paint(codeDim, "·"), r.paint(codeDim, msg))
	r.logf("note: %s", msg)
}

// Fatal reports an error that aborts the run.
func (r *Reporter) Fatal(msg string) {
	fmt.Fprintf(r.out, "\n  %s %s\n\n", r.paint(codeBold+codeRed, "Error:"), msg)
	r.logf("fatal: %s", msg)
}

// Done prints the closing summary line.
func (r *Reporter) Done() {
	elapsed := r.now().Sub(r.started).Round(100 * time.Millisecond)

	issueColor := codeDim
	if r.issues > 0 {
		issueColor = codeRed
	}

	verb := "written"
	if r.dryRun {
		verb = "to write"
	}

	segments := []string{
		r.paint(codeBold, "Done"),
		r.paint(codeGreen, fmt.Sprintf("%d %s", r.written, verb)),
		r.paint(codeDim, fmt.Sprintf("%d skipped", r.skipped)),
		r.paint(issueColor, fmt.Sprintf("%d %s", r.issues, plural(r.issues, "issue", "issues"))),
		r.paint(codeDim, elapsed.String()),
	}

	fmt.Fprintf(r.out, "  %s\n", r.paint(codeDim, separator))
	fmt.Fprintf(r.out, "  %s\n\n", strings.Join(segments, r.paint(codeDim, " · ")))
	r.logf("summary: %d %s, %d skipped, %d issues, %s", r.written, verb, r.skipped, r.issues, elapsed)
}

func (r *Reporter) paint(code, s string) string {
	if !r.color {
		return s
	}
	return code + s + codeReset
}

func (r *Reporter) logf(format string, args ...any) {
	if r.file == nil {
		return
	}
	fmt.Fprintf(r.file, "%s  %s\n", r.now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
