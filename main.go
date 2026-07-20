// Command plexmatch-generator writes .plexmatch hint files into every movie
// and show folder known to a Plex Media Server.
//
// It is a Go port of John Kidd Jr's PlexMatch-File-Generator (originally C#),
// built to run as a single static binary on a Raspberry Pi.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"plexmatch-generator/internal/cli"
	"plexmatch-generator/internal/generator"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		switch {
		case errors.Is(err, flag.ErrHelp):
			os.Exit(0)
		case errors.Is(err, cli.ErrVersion):
			fmt.Println(version)
			os.Exit(0)
		default:
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
	}

	os.Exit(generator.Run(context.Background(), opts))
}
