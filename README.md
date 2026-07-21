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

- A Plex server reachable over the network, and a Plex account to authorise
  against (see [Authentication](#authentication) below). You can also supply a
  token manually with `--token` if you prefer
  ([how to find it](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/)).
- To **build**: [Go](https://go.dev/dl/) 1.23 or newer. You don't need Go on the
  Raspberry Pi: cross-compile and copy the binary over.

## Download

Prebuilt static binaries are attached to every
[release](https://github.com/criscardozo/plexmatch-generator/releases/latest).
Nothing else needs to be installed on the Raspberry Pi.

First check your architecture:

```sh
uname -m
```

| `uname -m` | Binary to download |
| --- | --- |
| `aarch64` | `plexmatch-generator-linux-arm64` (Pi 3/4/5, 64-bit) |
| `armv7l` | `plexmatch-generator-linux-armv7` (Pi 2/3, 32-bit) |
| `armv6l` | `plexmatch-generator-linux-armv6` (Pi Zero / Pi 1) |
| `x86_64` | `plexmatch-generator-linux-amd64` |

Then download the matching binary (arm64 shown here), make it executable and run
it:

```sh
curl -L -o plexmatch-generator \
  https://github.com/criscardozo/plexmatch-generator/releases/latest/download/plexmatch-generator-linux-arm64
chmod +x plexmatch-generator
./plexmatch-generator
```

To install it system-wide so you can call it from anywhere:

```sh
sudo mv plexmatch-generator /usr/local/bin/
plexmatch-generator --version
```

> The `releases/latest/download/...` URL always points to the newest release, so
> the same command keeps working across versions.

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

The simplest way — no arguments. On the first run the tool asks you to authorise
the device with your Plex account and then discovers your server automatically:

```sh
./plexmatch-generator
```

You can still pass everything explicitly (handy for automated setups):

```sh
./plexmatch-generator --url http://192.168.0.3:32400 --token ABCD12345
# short form:
./plexmatch-generator -u http://192.168.0.3:32400 -t ABCD12345
```

### Authentication

You don't need to find your token by hand. On the first run (when no `--token`
is given and nothing is cached), the tool prints a URL like:

```
    https://app.plex.tv/auth#?clientID=...&code=...&context[device][product]=plexmatch-generator
```

Open it in any browser (your phone or laptop is fine), sign in to Plex and
approve the request. The tool detects the approval and caches the token in
`~/.config/plexmatch-generator/auth.json` (owner-only permissions), so you only
do this once. It works with two-factor authentication and never handles your
password.

- `--relogin` runs the flow again (for example after revoking the device).
- `--logout` deletes the cached token.
- `--token` still works if you'd rather provide one yourself; it overrides the cache.

### Server discovery

When `--url` is omitted, the tool asks your Plex account which servers you own
and picks a connection automatically (preferring a local one, ideal on a LAN).
If you own more than one server, use `--server-name "My Server"` to choose it
without a prompt, or pass `--url` directly.

### Options

| Option | Alias | Description |
| --- | --- | --- |
| `--url` | `-u` | Plex server URL (`http://` or `https://`). Optional — discovered automatically when omitted. |
| `--token` | `-t` | Plex token. Optional — obtained via login and cached when omitted; overrides the cache when given. |
| `--relogin` | | Ignore the cached token and authenticate again. |
| `--server-name` | | Pick a discovered server by name (used when `--url` is omitted). |
| `--logout` | | Delete the cached token and exit. |
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

Cron runs are non-interactive, so authenticate once by hand first (the token is
then cached and reused). After that you can drop `--token`; if you own multiple
servers, add `--server-name` or keep `--url` as shown.

## Project layout

```
main.go                      entry point / handles --version and --help
internal/cli/                argument parsing and validation
internal/plex/               Plex API HTTP client + JSON models
internal/plexauth/           plex.tv login (PIN flow), token cache, server discovery
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
