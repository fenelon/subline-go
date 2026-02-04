package main

import "fmt"

// PickAudioTrack selects an audio stream index from the probed tracks.
// If manual >= 0, it is used directly. Otherwise the function auto-selects:
//   - single track: returns it
//   - multiple tracks: returns the first and prints a hint about --audio-track
//   - no tracks: returns -1 and an error
func PickAudioTrack(tracks []AudioTrack, manual int) (int, error) {
	if manual >= 0 {
		fmt.Printf("  Using audio track %d (manual)\n", manual)
		return manual, nil
	}

	if len(tracks) == 0 {
		return -1, fmt.Errorf("no audio tracks found")
	}

	if len(tracks) == 1 {
		t := tracks[0]
		lang := t.Language
		if lang == "" {
			lang = "und"
		}
		fmt.Printf("  Using audio track %d (%s)\n", t.StreamIndex, lang)
		return t.StreamIndex, nil
	}

	// Multiple tracks: use first, hint about --audio-track.
	fmt.Print("  Audio tracks: [")
	for i, t := range tracks {
		lang := t.Language
		if lang == "" {
			lang = "und"
		}
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("(%d, %s)", t.StreamIndex, lang)
	}
	fmt.Println("]")

	t := tracks[0]
	lang := t.Language
	if lang == "" {
		lang = "und"
	}
	fmt.Printf("  Using first track %d (%s). Override with --audio-track N\n", t.StreamIndex, lang)
	return t.StreamIndex, nil
}
