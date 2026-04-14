# go-media-manage

A CLI replacement for TinyMediaManager. Point it at a directory of TV shows or movies and it will match against [TMDB](https://www.themoviedb.org/), download artwork, and write [Kodi](https://kodi.tv/)-compatible NFO files — no GUI required.

## Features

- Auto-detects TV shows and movies from filenames
- Searches TMDB and prompts you to confirm when multiple matches exist
- Caches matches locally so repeat runs skip the API lookup
- Writes Kodi-compatible NFO files for shows, seasons, episodes, and movies
- Downloads poster, fanart, season posters, and episode thumbnails
- `--dry-run` mode to preview what would be written

## Output layout

**TV show:**
```
/media/TV/Breaking Bad/
├── tvshow.nfo
├── poster.jpg
├── fanart.jpg
├── season01-poster.jpg
├── season02-poster.jpg
├── Season 1/
│   ├── season.nfo
│   ├── Breaking.Bad.S01E01.mkv
│   ├── Breaking.Bad.S01E01.nfo
│   ├── Breaking.Bad.S01E01-thumb.jpg
│   └── ...
└── Season 2/
    └── ...
```

**Movie:**
```
/media/Movies/Inception (2010)/
├── Inception.2010.mkv
├── movie.nfo
├── poster.jpg
└── fanart.jpg
```

## Installation

Requires Go 1.26+.

```sh
git clone https://github.com/your-username/go-media-manage
cd go-media-manage
go build -o go-media-manage .
```

Or install directly:

```sh
go install go-media-manage@latest
```

## Setup

Get a free Read Access Token from [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api) (under the "API Read Access Token" section), then:

```sh
go-media-manage config set-token YOUR_READ_ACCESS_TOKEN
```

## Usage

```sh
# Scan a TV show directory (auto-detected)
go-media-manage scan /media/TV/BreakingBad

# Force type if auto-detect gets it wrong
go-media-manage scan /media/TV/Succession --type tv
go-media-manage scan /media/Movies/Dune --type movie

# Preview without writing any files
go-media-manage scan /media/TV/TheWire --dry-run

# Re-fetch metadata even if already cached
go-media-manage scan /media/TV/Sopranos --force

# Change metadata language
go-media-manage config set-language de-DE
```

### Interactive matching

When TMDB returns multiple results you'll be prompted to pick:

```
Multiple results — pick one:
  [1] Breaking Bad (2008) — TMDB 1396
  [2] Breaking Bad (2012) — TMDB 99999
  [0] None / cancel
> 1
```

## Supported filename formats

**TV episodes:**
- `Show.Name.S01E02.mkv`
- `Show Name - s01e02 - Episode Title.mkv`
- `Show_Name_1x02.mkv`

**Movies:**
- `Movie Title (2020).mkv`
- `Movie.Title.2020.1080p.mkv`

## Configuration

Config is stored at `~/.config/go-media-manage/config.json`. The match cache (`matches.json`) is written directly into the scanned directory, alongside the media files.

```sh
go-media-manage config show
```

```
TMDB token : abcd****ef12
Language   : en-US
```

## NFO format

NFO files follow the Kodi/XBMC schema and are compatible with Jellyfin, Emby, and any other media server that reads Kodi metadata.
