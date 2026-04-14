package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type MediaType string

const (
	TypeTV    MediaType = "tv"
	TypeMovie MediaType = "movie"
	TypeAuto  MediaType = "auto"
)

// MediaFile represents a single discovered media file.
type MediaFile struct {
	Path    string
	Dir     string
	Base    string // filename without extension
	Ext     string
	Type    MediaType
	Title   string
	Year    int
	Season  int
	Episode int
}

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true,
	".mov": true, ".wmv": true, ".m4v": true,
	".ts": true, ".m2ts": true,
}

// TV patterns: matches S01E02, 1x02, s01e02, etc.
var tvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[._\s-]s(\d{1,2})e(\d{1,2})`),
	regexp.MustCompile(`(?i)[._\s-](\d{1,2})x(\d{2})`),
	regexp.MustCompile(`(?i)[._\s-](\d{1,2})x(\d{2})[._\s-]`),
}

// Movie year pattern
var yearPattern = regexp.MustCompile(`\((\d{4})\)|[._\s](\d{4})[._\s]`)

// cleanTitle converts file path separators and dots/underscores to spaces, strips junk.
func cleanTitle(raw string) string {
	// Remove everything from the season/episode marker onward
	for _, re := range tvPatterns {
		if loc := re.FindStringIndex(strings.ToLower(raw)); loc != nil {
			raw = raw[:loc[0]]
		}
	}
	// Remove year patterns
	if loc := yearPattern.FindStringIndex(raw); loc != nil {
		raw = raw[:loc[0]]
	}
	// Replace dots, underscores, hyphens with spaces
	r := strings.NewReplacer(".", " ", "_", " ", "-", " ")
	raw = r.Replace(raw)
	// Collapse multiple spaces
	raw = strings.Join(strings.Fields(raw), " ")
	return strings.TrimSpace(raw)
}

func ParseFile(path string, hint MediaType) (*MediaFile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if !videoExts[ext] {
		return nil, fmt.Errorf("not a video file")
	}

	base := filepath.Base(path)
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))

	mf := &MediaFile{
		Path: path,
		Dir:  filepath.Dir(path),
		Base: baseNoExt,
		Ext:  ext,
	}

	// Try TV patterns first (unless told it's a movie)
	if hint != TypeMovie {
		for _, re := range tvPatterns {
			m := re.FindStringSubmatch(baseNoExt)
			if m != nil {
				s, _ := strconv.Atoi(m[1])
				e, _ := strconv.Atoi(m[2])
				mf.Type = TypeTV
				mf.Season = s
				mf.Episode = e
				mf.Title = cleanTitle(baseNoExt)
				return mf, nil
			}
		}
	}

	// Fall back to movie
	mf.Type = TypeMovie
	if m := yearPattern.FindStringSubmatch(baseNoExt); m != nil {
		for i := 1; i < len(m); i++ {
			if m[i] != "" {
				mf.Year, _ = strconv.Atoi(m[i])
				break
			}
		}
	}
	mf.Title = cleanTitle(baseNoExt)
	return mf, nil
}

// ScanResult holds everything found under a directory.
type ScanResult struct {
	RootDir   string
	MediaType MediaType
	Title     string // inferred show/movie title
	Year      int
	Files     []*MediaFile
}

// Scan walks dir and returns grouped results.
func Scan(dir string, hint MediaType) (*ScanResult, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("accessing directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	result := &ScanResult{RootDir: dir}
	var files []*MediaFile

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		mf, err := ParseFile(path, hint)
		if err != nil {
			return nil // not a video file, skip
		}
		files = append(files, mf)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no video files found in %s", dir)
	}

	result.Files = files

	// Determine overall type: if any file parsed as TV, treat as TV
	tvCount, movieCount := 0, 0
	for _, f := range files {
		if f.Type == TypeTV {
			tvCount++
		} else {
			movieCount++
		}
	}

	if hint == TypeTV || (hint == TypeAuto && tvCount > movieCount) {
		result.MediaType = TypeTV
		// Title comes from the root directory name (most reliable for TV)
		result.Title = cleanTitle(filepath.Base(dir))
	} else {
		result.MediaType = TypeMovie
		result.Title = files[0].Title
		result.Year = files[0].Year
	}

	return result, nil
}
