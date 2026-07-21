package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestReporterPlainOutput(t *testing.T) {
	t.Parallel()

	var out, file bytes.Buffer
	fixed := time.Date(2026, 7, 21, 17, 0, 0, 0, time.UTC)
	r := NewWithClock(&out, &file, false, false, func() time.Time { return fixed })

	r.Start("ObiWan", "https://x:32400/")
	r.Wrote("About Time")
	r.WroteSeason("Firefly", 1)
	r.Warn("All Inclusive — no media found")
	r.Skip()
	r.Done()

	got := out.String()
	for _, want := range []string{
		"Plex", "ObiWan",
		"✓ About Time",
		"season 1",
		"! All Inclusive",
		"Done", "2 written", "1 skipped", "1 issue",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("console output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "\033[") {
		t.Errorf("expected no ANSI codes when colour is off, got:\n%q", got)
	}

	logged := file.String()
	if !strings.Contains(logged, "2026-07-21T17:00:00Z") || !strings.Contains(logged, `wrote "About Time"`) {
		t.Errorf("file log missing timestamp or message:\n%s", logged)
	}
}

func TestReporterColorEmitsANSI(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	fixed := time.Unix(0, 0).UTC()
	r := NewWithClock(&out, nil, true, false, func() time.Time { return fixed })

	r.Wrote("X")
	if !strings.Contains(out.String(), "\033[32m") {
		t.Errorf("expected a green ANSI code, got %q", out.String())
	}
}

func TestReporterDryRunOutput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	fixed := time.Unix(0, 0).UTC()
	r := NewWithClock(&out, nil, false, true, func() time.Time { return fixed })

	r.Wrote("About Time")
	r.Done()

	got := out.String()
	for _, want := range []string{"~ About Time", "1 to write"} {
		if !strings.Contains(got, want) {
			t.Errorf("dry-run output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "✓") {
		t.Errorf("dry-run should not use the ✓ symbol:\n%s", got)
	}
}
