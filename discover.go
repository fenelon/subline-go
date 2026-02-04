package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MediaExts is the set of file extensions recognised as media (video + audio).
var MediaExts = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".wav":  true,
	".flac": true,
	".mp3":  true,
}

// FindMediaFiles returns all media files found in the given paths.
// Each path can be a direct file or a directory (contents are listed sorted).
// Non-existent or unrecognised paths produce a warning on stderr.
func FindMediaFiles(paths []string) []string {
	var found []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping '%s' (not a file or directory)\n", p)
			continue
		}
		if info.Mode().IsRegular() {
			if MediaExts[strings.ToLower(filepath.Ext(p))] {
				found = append(found, p)
			}
			continue
		}
		if info.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping '%s' (%v)\n", p, err)
				continue
			}
			// os.ReadDir returns entries sorted by name already, but
			// we sort explicitly to match the Python behaviour exactly.
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				if e.Type().IsRegular() {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)
			for _, name := range names {
				if MediaExts[strings.ToLower(filepath.Ext(name))] {
					found = append(found, filepath.Join(p, name))
				}
			}
			continue
		}
		// Not a regular file and not a directory (e.g. symlink, device, etc.)
		fmt.Fprintf(os.Stderr, "Warning: skipping '%s' (not a file or directory)\n", p)
	}
	return found
}
