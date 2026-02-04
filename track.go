package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// clearLines moves the cursor up n lines and clears each one.
func clearLines(n int) {
	for i := 0; i < n; i++ {
		fmt.Print("\033[A\033[2K")
	}
}

// PickAudioTracks selects audio stream indices from the probed tracks.
// If manual >= 0, it is used directly. Otherwise:
//   - single track: returns it
//   - multiple tracks: presents an interactive menu with an "all" option
//   - no tracks: returns nil and an error
func PickAudioTracks(tracks []AudioTrack, manual int) ([]int, error) {
	if manual >= 0 {
		fmt.Printf("  Using audio track %d (manual)\n", manual)
		return []int{manual}, nil
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks found")
	}

	if len(tracks) == 1 {
		t := tracks[0]
		lang := t.Language
		if lang == "" {
			lang = "und"
		}
		fmt.Printf("  Using audio track %d (%s)\n", t.StreamIndex, lang)
		return []int{t.StreamIndex}, nil
	}

	// Multiple tracks: interactive selection.
	// Track lines printed so we can clear the menu after selection.
	menuLines := 0

	fmt.Println("  Multiple audio tracks found:")
	menuLines++
	for i, t := range tracks {
		lang := t.Language
		if lang == "" {
			lang = "und"
		}
		fmt.Printf("    %d) stream %d — %s (%s, %dch, %dHz)\n",
			i+1, t.StreamIndex, lang, t.Codec, t.Channels, t.SampleRate)
		menuLines++
	}
	fmt.Printf("    a) all tracks\n")
	menuLines++

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("  Select track [1-%d or a]: ", len(tracks))
		menuLines++
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if strings.EqualFold(line, "a") || strings.EqualFold(line, "all") {
			clearLines(menuLines)
			fmt.Println("  Using all audio tracks")
			indices := make([]int, len(tracks))
			for i, t := range tracks {
				indices[i] = t.StreamIndex
			}
			return indices, nil
		}

		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(tracks) {
			clearLines(menuLines)
			t := tracks[n-1]
			lang := t.Language
			if lang == "" {
				lang = "und"
			}
			fmt.Printf("  Using audio track %d (%s)\n", t.StreamIndex, lang)
			return []int{t.StreamIndex}, nil
		}

		fmt.Printf("  Invalid choice. Enter a number between 1 and %d, or 'a' for all.\n", len(tracks))
		menuLines++
	}
}

// TrackLanguage returns the language for a given stream index from the tracks list,
// or "und" if not found.
func TrackLanguage(tracks []AudioTrack, streamIndex int) string {
	for _, t := range tracks {
		if t.StreamIndex == streamIndex {
			if t.Language != "" {
				return t.Language
			}
			return "und"
		}
	}
	return "und"
}
