# plexmatch-generator

A command-line tool that generates a `.plexmatch` file in the folder of every
movie and show known to your **Plex Media Server**. This is handy, for example,
when migrating storage devices and you want to preserve the custom matches you
already had.

This project is a **rewrite in [Go](https://go.dev/)** of John Kidd Jr's original
**PlexMatch File Generator**, written in C#:

- Original project: **PlexMatch-File-Generator**
- Original repository: <https://github.com/johnkiddjr/PlexMatch-File-Generator>

The goal of the port is to run as a **single static binary, with no runtime or
dependencies**, ideal for a **Raspberry Pi** running Linux.

## What does it do?

It doesn't scan your disk: it connects to your **Plex server's HTTP API** (using
your token), walks the libraries and, for each media folder, writes a
`.plexmatch` file with the match information Plex resolved. More about the
format: <https://support.plex.tv/articles/plexmatch/>.

Example of the file generated for a movie or the root of a show:

```
Title: Firefly
Year: 2002
Guid: plex://show/5d9c081b170e6d001f8a8e0f
```

And for a season folder (when applicable):

```
Title: Firefly
Year: 2002
Season: 1
Guid: plex://season/602e67d4b16bd9002d5f7f
```

## Requirements

- A Plex server reachable over the network and its **token** (`X-Plex-Token`).
  How to find it: <https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/>
- To **build**: [Go](https://go.dev/dl/) 1.23 or newer. You don't need Go on the
  Raspberry Pi: cross-compile and copy the binary over.

## Build

For the Raspberry Pi (64-bit, Pi 3/4/5 running 64-bit Raspberry Pi OS):

```sh
make rpi          # produces bin/plexmatch-generator-linux-arm64
```

Other Raspberry Pi targets:

```sh
make rpi32        # 32-bit (Pi 2/3, ARMv7)  -> bin/plexmatch-generator-linux-armv7
make rpi-zero     # Pi Zero / Pi 1 (ARMv6)  -> bin/plexmatch-generator-linux-armv6
make release      # builds all three targets above
```

For your current machine (for example, to test locally):

```sh
make build        # produces bin/plexmatch-generator
make test         # runs the tests (with the race detector)
```

Copy the binary to the Raspberry Pi (for example with `scp`) and make it
executable:

```sh
scp bin/plexmatch-generator-linux-arm64 pi@raspberrypi:~/plexmatch-generator
ssh pi@raspberrypi 'chmod +x ~/plexmatch-generator'
```

> The cross-compile uses `CGO_ENABLED=0`, so the binary is static and doesn't
> depend on any system libraries.

## Usage

```sh
./plexmatch-generator --url http://192.168.0.3:32400 --token ABCD12345
```

Equivalent short form:

```sh
./plexmatch-generator -u http://192.168.0.3:32400 -t ABCD12345
```

If you omit `--url` or `--token`, the tool prompts you for them.

### Options

| Option | Alias | Description |
| --- | --- | --- |
| `--url` | `-u` | Plex server URL (must start with `http://` or `https://`). **Required.** |
| `--token` | `-t` | Plex authentication token. **Required.** |
| `--root` | `-r` | Remap a Plex path to the host path, in the form `hostPath:plexPath`. Repeatable. |
| `--library` | `-lib` | Only process this library (by name). Repeatable. Case-insensitive. |
| `--show` | `-s` | Only process this item (by title). Repeatable. Case-insensitive. |
| `--seasonprocessing` | `-sp` | Also write a `.plexmatch` in every season folder. |
| `--nooverwrite` | `-no` | Don't overwrite folders that already have a `.plexmatch`. |
| `--pagesize` | `-ps` | Number of items per page when walking a library (default: 20). |
| `--log` | `-l` | In addition to the console, write the log to `<dir>/plexmatch.log`. The directory must exist. |
| `--version` | | Print the version and exit. |
| `--help` | `-h` | Show help. |

### Path remapping (`--root`)

Use this when Plex sees paths differently from the host running this tool. For
example, if Plex runs in a container that mounts the media at `/media` but on the
Raspberry Pi it lives at `/mnt/media`:

```sh
./plexmatch-generator -u http://192.168.0.3:32400 -t ABCD12345 -r /mnt/media:/media
```

The form is `hostPath:plexPath`: the first path is the host's (where the file is
written) and the second is the one Plex reports. Remapping is applied by prefix.

### Filtering what gets processed

```sh
# Only the "TV Shows" library
./plexmatch-generator -u http://... -t ... -lib "TV Shows"

# Only a specific show (best combined with -lib on large libraries)
./plexmatch-generator -u http://... -t ... -lib "TV Shows" -s firefly
```

### Per-season processing

By default, for shows with an episode ordering other than the library default
(absolute / aired / dvd / tmdb) a `.plexmatch` is also written into each season
folder to preserve that ordering. With `--seasonprocessing` this behaviour is
forced for every show.

## Scheduled runs (optional)

If you want it to run periodically on the Raspberry Pi, add an entry to
`crontab -e`, for example every day at 4 AM:

```
0 4 * * * /home/pi/plexmatch-generator -u http://192.168.0.3:32400 -t ABCD12345 --log /home/pi/logs
```

## Project layout

```
main.go                      entry point / handles --version and --help
internal/cli/                argument parsing and validation
internal/plex/               Plex API HTTP client + JSON models
internal/plexmatch/          .plexmatch file format and writing
internal/generator/          core logic (traversal, remapping and writing)
Makefile                     build targets (including ARM cross-compile)
```

## Credits

- Original project and format: **John Kidd Jr** —
  <https://github.com/johnkiddjr/PlexMatch-File-Generator>
- `.plexmatch` format: <https://support.plex.tv/articles/plexmatch/>

## Licence

MIT. See [LICENSE](LICENSE). The original copyright of John Kidd Jr is preserved
alongside that of the Go port.
