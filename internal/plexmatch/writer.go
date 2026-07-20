// Package plexmatch writes .plexmatch hint files in the exact line format Plex
// expects. See https://support.plex.tv/articles/plexmatch/
package plexmatch

import (
	"os"
	"strconv"
	"strings"
)

// FileName is the fixed name Plex looks for.
const FileName = ".plexmatch"

// Info is the data written into a single .plexmatch file.
type Info struct {
	Title    string
	Year     int
	Guid     string
	Season   int
	IsSeason bool // when true, a "Season:" line is written and Guid is the season GUID
}

// Render returns the exact file contents for info. The field order matters to
// Plex: Title, Year, [Season], Guid, each followed by a newline.
func Render(info Info) string {
	var b strings.Builder
	b.WriteString("Title: " + info.Title + "\n")
	b.WriteString("Year: " + strconv.Itoa(info.Year) + "\n")
	if info.IsSeason {
		b.WriteString("Season: " + strconv.Itoa(info.Season) + "\n")
	}
	b.WriteString("Guid: " + info.Guid + "\n")
	return b.String()
}

// Write renders info and writes it to path, overwriting any existing file
// (UTF-8, no BOM), matching the original tool's behaviour.
func Write(path string, info Info) error {
	return os.WriteFile(path, []byte(Render(info)), 0o644)
}
