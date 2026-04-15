package scope

import (
	"os"
	"path/filepath"

	"go-media-manage/internal/scanner"
)

// Scope controls which part of the media tree to operate on.
type Scope struct {
	all    bool
	root   bool
	season int // >0 means a specific season number
}

// FromDir infers a Scope from the directory path.
// If the directory name matches a season pattern (e.g. "Season 01", "S02"),
// returns a season-specific Scope. Otherwise returns an all-inclusive Scope.
func FromDir(dir string) Scope {
	if n := scanner.ParseSeasonDir(dir); n > 0 {
		return Scope{season: n}
	}
	return Scope{all: true}
}

// Root returns a root-only Scope (show-level files; no seasons or episodes).
func Root() Scope {
	return Scope{root: true}
}

// RootDir returns the show root directory for the given target path.
// For a season-specific Scope the show root is the parent of dir;
// for all other scopes it is dir itself.
func (sc Scope) RootDir(dir string) string {
	if sc.season > 0 {
		return filepath.Dir(dir)
	}
	return dir
}

// IncludesRoot reports whether show-level files (tvshow.nfo, poster, etc.) are in scope.
func (sc Scope) IncludesRoot() bool { return sc.all || sc.root }

// IncludesSeason reports whether the given season number is in scope.
func (sc Scope) IncludesSeason(n int) bool { return sc.all || sc.season == n }

// Season returns the specific season number, or 0 if the scope is not season-specific.
func (sc Scope) Season() int { return sc.season }

// Files returns the subset of files whose season falls within the scope.
func (sc Scope) Files(files []*scanner.MediaFile) []*scanner.MediaFile {
	out := make([]*scanner.MediaFile, 0, len(files))
	for _, f := range files {
		if sc.IncludesSeason(f.Season) {
			out = append(out, f)
		}
	}
	return out
}

// Seasons returns the sorted distinct season numbers from files that fall within scope.
func (sc Scope) Seasons(files []*scanner.MediaFile) []int {
	return scanner.UniqueSeasons(sc.Files(files))
}

// WalkDir walks dir, calling fn for each entry that falls within the scope.
// fn is called for both files and in-scope directories (excluding dir itself).
// Season directories outside the scope and the "archive" subdirectory are always skipped.
func (sc Scope) WalkDir(dir string, fn func(path string, d os.DirEntry) error) error {
	archiveDir := filepath.Join(dir, "archive")
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch {
			case path == archiveDir:
				return filepath.SkipDir
			case path == dir:
				return nil
			case sc.root:
				return filepath.SkipDir
			case sc.season > 0 && scanner.ParseSeasonDir(path) != sc.season:
				return filepath.SkipDir
			}
		}
		return fn(path, d)
	})
}
