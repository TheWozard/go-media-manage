package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/scanner"
)

var (
	flagSwapImage    bool
	flagSwapMetadata bool
)

var swapCmd = &cobra.Command{
	Use:   "swap <directory> <epA> <epB>",
	Short: "Swap media files between two episodes",
	Long: `Swaps the video files for two episodes. Use --image to also swap
thumbnails and --metadata to also swap NFO files.

Episode format: s1e2 or S01E04`,
	Args: cobra.ExactArgs(3),
	RunE: runSwap,
}

func init() {
	swapCmd.Flags().BoolVar(&flagSwapImage, "image", false, "Also swap thumbnail images")
	swapCmd.Flags().BoolVar(&flagSwapMetadata, "metadata", false, "Also swap NFO metadata files")
	rootCmd.AddCommand(swapCmd)
}

var epSpecPattern = regexp.MustCompile(`(?i)^s(\d{1,3})e(\d{1,3})$`)

func parseEpSpec(s string) (season, episode int, err error) {
	m := epSpecPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("invalid episode spec %q — expected format like s1e2 or S01E04", s)
	}
	season, _ = strconv.Atoi(m[1])
	episode, _ = strconv.Atoi(m[2])
	return
}

func runSwap(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	sA, eA, err := parseEpSpec(args[1])
	if err != nil {
		return err
	}

	videoA, err := findEpisodeVideo(dir, sA, eA)
	if err != nil {
		return fmt.Errorf("episode %s: %w", args[1], err)
	}

	var videoB string
	if sB, eB, err := parseEpSpec(args[2]); err == nil {
		if sA == sB && eA == eB {
			return fmt.Errorf("both episode specs are the same")
		}
		videoB, err = findEpisodeVideo(dir, sB, eB)
		if err != nil {
			return fmt.Errorf("episode %s: %w", args[2], err)
		}
	} else {
		videoB, err = findEpisodeVideoByName(dir, args[2])
		if err != nil {
			return fmt.Errorf("searching for %q: %w", args[2], err)
		}
		if videoA == videoB {
			return fmt.Errorf("search matched the same file as %s", args[1])
		}
	}

	baseA := strings.TrimSuffix(filepath.Base(videoA), filepath.Ext(videoA))
	baseB := strings.TrimSuffix(filepath.Base(videoB), filepath.Ext(videoB))
	dirA := filepath.Dir(videoA)
	dirB := filepath.Dir(videoB)

	pairs := []struct{ a, b string }{
		{videoA, videoB},
	}

	if flagSwapMetadata {
		nfoA := filepath.Join(dirA, baseA+".nfo")
		nfoB := filepath.Join(dirB, baseB+".nfo")
		if fileExists(nfoA) && fileExists(nfoB) {
			pairs = append(pairs, struct{ a, b string }{nfoA, nfoB})
		} else {
			fmt.Fprintln(os.Stderr, "  warning: NFO file(s) not found, skipping metadata swap")
		}
	}

	if flagSwapImage {
		thumbA := filepath.Join(dirA, baseA+"-thumb.jpg")
		thumbB := filepath.Join(dirB, baseB+"-thumb.jpg")
		if fileExists(thumbA) && fileExists(thumbB) {
			pairs = append(pairs, struct{ a, b string }{thumbA, thumbB})
		} else {
			fmt.Fprintln(os.Stderr, "  warning: thumb file(s) not found, skipping image swap")
		}
	}

	for _, p := range pairs {
		if err := swapFiles(p.a, p.b); err != nil {
			return err
		}
	}

	fmt.Printf("\nSwapped %d file(s) between %s and %s.\n", len(pairs), args[1], args[2])
	return nil
}

// findEpisodeVideo walks dir to find the video file for the given season/episode.
func findEpisodeVideo(dir string, season, episode int) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		mf, parseErr := scanner.ParseFile(path, scanner.TypeTV)
		if parseErr != nil {
			return nil
		}
		if mf.Season == season && mf.Episode == episode {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no video file found for S%02dE%02d in %s", season, episode, dir)
	}
	return found, nil
}

// findEpisodeVideoByName searches dir for video files whose base name contains
// query (case-insensitive). If exactly one match is found it is returned. If
// multiple matches are found the user is prompted to pick one.
func findEpisodeVideoByName(dir, query string) (string, error) {
	var matches []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !scanner.VideoExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if strings.Contains(strings.ToLower(base), strings.ToLower(query)) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no video file matching %q found in %s", query, dir)
	case 1:
		return matches[0], nil
	default:
		fmt.Printf("Multiple files match %q:\n", query)
		for i, m := range matches {
			fmt.Printf("  [%d] %s\n", i+1, filepath.Base(m))
		}
		n, err := readInt("> ", 1, len(matches))
		if err != nil {
			return "", err
		}
		return matches[n-1], nil
	}
}

// swapFiles exchanges two files via a temp name.
func swapFiles(a, b string) error {
	tmp := a + ".swaptmp"
	fmt.Printf("  %s\n  ↔ %s\n", filepath.Base(a), filepath.Base(b))
	if err := os.Rename(a, tmp); err != nil {
		return fmt.Errorf("renaming %s to tmp: %w", filepath.Base(a), err)
	}
	if err := os.Rename(b, a); err != nil {
		os.Rename(tmp, a) // attempt restore
		return fmt.Errorf("renaming %s: %w", filepath.Base(b), err)
	}
	if err := os.Rename(tmp, b); err != nil {
		return fmt.Errorf("renaming tmp to %s: %w", filepath.Base(b), err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
