package scanner

import "sort"

// UniqueSeasons returns a sorted slice of distinct season numbers in files.
func UniqueSeasons(files []*MediaFile) []int {
	seen := make(map[int]bool)
	for _, f := range files {
		seen[f.Season] = true
	}
	seasons := make([]int, 0, len(seen))
	for s := range seen {
		seasons = append(seasons, s)
	}
	sort.Ints(seasons)
	return seasons
}

// GroupBySeasonDir groups files by their parent directory path.
func GroupBySeasonDir(files []*MediaFile) map[string][]*MediaFile {
	m := make(map[string][]*MediaFile)
	for _, f := range files {
		m[f.Dir] = append(m[f.Dir], f)
	}
	return m
}
