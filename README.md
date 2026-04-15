# go-media-manage

A CLI replacement for TinyMediaManager. Point it at a directory of TV shows or movies and it will match against [TMDB](https://www.themoviedb.org/), download artwork, write [Kodi](https://kodi.tv/)-compatible NFO files, and rename your files — no GUI required.

## Features

- Three-stage workflow: `match` → `pull` → `rename`
- Auto-detects TV shows and movies from filenames and directory structure
- Searches TMDB and prompts you to confirm when multiple matches exist
- Optional episode group selection for alternative orderings (absolute, DVD, etc.)
- Caches the match in `matches.json` inside each media directory
- Writes Kodi-compatible NFO files for shows, seasons, episodes, and movies
- Downloads poster, fanart, season posters, and episode thumbnails
- Renames video files and NFOs to a clean standard format using NFO metadata

## Workflow

```sh
# 1. Match — interactive TMDB lookup, saves matches.json
go-media-manage match /media/TV/Breaking\ Bad

# 2. Pull — fetch metadata and artwork (scoped, opt-in)
go-media-manage pull /media/TV/Breaking\ Bad all --metadata --images

# 3. Rename — rename files using the written NFO metadata
go-media-manage rename /media/TV/Breaking\ Bad
```

## Output layout

**TV show:**
```
/media/TV/Breaking Bad/
├── tvshow.nfo
├── poster.jpg
├── fanart.jpg
├── season01-poster.jpg
├── Season 01/
│   ├── season.nfo
│   ├── Breaking Bad S01E01 - Pilot.mkv
│   ├── Breaking Bad S01E01 - Pilot.nfo
│   ├── Breaking Bad S01E01 - Pilot-thumb.jpg
│   └── ...
└── Season 02/
    └── ...
```

**Movie:**
```
/media/Movies/Inception (2010)/
├── Inception (2010).mkv
├── movie.nfo
├── poster.jpg
└── fanart.jpg
```

## Installation

Requires Go 1.26+.

```sh
git clone https://github.com/your-username/go-media-manage
cd go-media-manage
go install .
```

This places the `go-media-manage` binary in `$GOPATH/bin` (usually `~/go/bin`). Make sure that directory is on your `$PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Setup

Get a free Read Access Token from [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api) (under the "API Read Access Token" section), then:

```sh
go-media-manage config set-token YOUR_READ_ACCESS_TOKEN
```

## Commands

### `match`

Scans a directory, searches TMDB, and saves the result to `matches.json`. Always re-matches, overwriting any existing cache.

```sh
go-media-manage match <directory> [flags]

Flags:
  -t, --type string   Media type: auto, tv, movie (default "auto")
```

When TMDB returns multiple results you'll be prompted to pick one:

```
Multiple results — pick one:
  [1] Breaking Bad (2008) — TMDB 1396
  [2] Breaking Bad (2012) — TMDB 99999
  [0] None / cancel
> 1
```

For TV shows, if TMDB has alternative episode groups (absolute order, DVD order, etc.) you'll be offered a choice:

```
Episode groups available — pick one (or 0 for standard ordering):
  [1] Absolute Order (Absolute, 226 episodes)
  [2] DVD Order (DVD, 200 episodes)
  [0] None / use standard ordering
> 0
```

### `pull`

Reads `matches.json` and downloads metadata and artwork for the given scope. Errors if `match` hasn't been run yet.

```sh
go-media-manage pull <directory> <scope> [flags]

Scope:
  all       Everything — show root, all seasons, and all episodes
  root      Show-level only (tvshow.nfo, poster, fanart)
  s1,s2,…   A single season (season.nfo, season poster, episode NFOs/thumbs)

Flags:
  --metadata      Write NFO files
  --images        Download missing artwork
  --all-images    Download all artwork, replacing existing files
```

At least one of `--metadata`, `--images`, or `--all-images` must be provided.

**Examples:**

```sh
# Full first-time pull
go-media-manage pull /media/TV/Breaking\ Bad all --metadata --images

# Re-fetch metadata only for season 2
go-media-manage pull /media/TV/Breaking\ Bad s2 --metadata

# Force re-download all artwork
go-media-manage pull /media/TV/Breaking\ Bad all --all-images
```

### `rename`

Reads the NFO files written by `pull` and renames the matching video, NFO, and thumbnail to a clean standard format.

```sh
go-media-manage rename <directory>
```

**TV:** `Show Name - S01E01 - Episode Title.mkv`  
**Movie:** `Movie Title (2010).mkv`

Season directories are renamed to `Season 01`, `Season 02`, etc.

Characters illegal on common filesystems (`:`, `*`, `?`, etc.) are replaced with `-`.

### `cleanup`

Moves all non-MKV files (NFOs, JPGs, JSONs, etc.) into an `archive/` subfolder at the root of the directory, preserving relative paths.

```sh
go-media-manage cleanup <directory>
```

### `config`

```sh
go-media-manage config set-token <read-access-token>   # set TMDB token
go-media-manage config set-language <lang>             # e.g. de-DE
go-media-manage config show                            # print current config
```

## Supported filename formats

The scanner recognises these patterns without any configuration:

**TV episodes:**
- `Show.Name.S01E02.mkv`
- `Show Name - s01e02 - Episode Title.mkv`
- `Show_Name_1x02.mkv`
- `1.mkv`, `2.mkv` … inside a `Season 01/` directory

**Movies:**
- `Movie Title (2020).mkv`
- `Movie.Title.2020.1080p.mkv`

## Configuration

Config is stored at `~/.config/go-media-manage/config.json`. The match cache (`matches.json`) is written directly into each scanned directory and travels with the media files.

```
TMDB token : abcd****ef12
Language   : en-US
```

## NFO format

NFO files follow the Kodi/XBMC schema and are compatible with Jellyfin, Emby, and any media server that reads Kodi metadata.
