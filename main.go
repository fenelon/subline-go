package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	// Banner.
	fmt.Printf("\n\033[1m"+
		"░▄▀▀░█▒█░██▄░█▒░░█░█▄░█▒██▀\n"+
		"▒▄██░▀▄█▒█▄█▒█▄▄░█░█▒▀█░█▄▄\n"+
		"\033[0m"+
		"Subline %s - AI subtitles made easy\n\n", Version)

	// Parse flags.
	language := flag.String("language", "", "Language code (auto-detect if omitted)")
	model := flag.String("model", "turbo", "Whisper model (tiny/base/small/medium/turbo/large)")
	audioTrack := flag.Int("audio-track", -1, "Audio stream index (-1 = auto-detect)")
	format := flag.String("format", "srt", "Output format: srt or vtt")
	outputDir := flag.String("output-dir", "", "Directory to write subtitle files (default: next to source)")
	skipExisting := flag.Bool("skip-existing", false, "Skip files that already have a subtitle file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: subline [options] <path...>\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *format != "srt" && *format != "vtt" {
		fmt.Fprintf(os.Stderr, "Error: --format must be 'srt' or 'vtt'\n")
		os.Exit(1)
	}

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Find media files.
	files := FindMediaFiles(paths)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No media files found in: %s\n", strings.Join(paths, " "))
		os.Exit(1)
	}

	// Device info.
	device := "cpu"
	if runtime.GOOS == "darwin" {
		device = "Metal"
	}

	langStr := *language
	if langStr == "" {
		langStr = "auto-detect"
	}
	fmt.Printf("Found %d file(s) | model=%s | language=%s | device=%s\n\n", len(files), *model, langStr, device)

	// Ensure model is downloaded.
	fmt.Fprintf(os.Stderr, "Loading model '%s'...\n", *model)
	modelPath, err := EnsureModel(*model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load whisper model.
	wm, err := LoadModel(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		os.Exit(1)
	}
	defer wm.Close()
	fmt.Println()

	// Create output directory if specified.
	if *outputDir != "" {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Signal handling: clean up partial output on interrupt.
	var currentOutput string
	cancelSignal := SignalCleanup(&currentOutput)
	defer cancelSignal()

	// Process each file.
	for vi, file := range files {
		fmt.Printf("[%d/%d] %s\n", vi+1, len(files), filepath.Base(file))

		// Determine output path.
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		outName := base + "." + *format
		var outPath string
		if *outputDir != "" {
			outPath = filepath.Join(*outputDir, outName)
		} else {
			outPath = filepath.Join(filepath.Dir(file), outName)
		}

		// Skip existing.
		if *skipExisting {
			if _, err := os.Stat(outPath); err == nil {
				fmt.Println("  Skipping (subtitle file exists)")
				continue
			}
		}

		// Probe audio tracks.
		tracks, err := ProbeAudioTracks(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error probing audio: %v\n", err)
			continue
		}

		// Pick audio track.
		streamIdx, err := PickAudioTrack(tracks, *audioTrack)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %v, skipping\n", err)
			continue
		}

		// Extract audio.
		fmt.Printf("  Extracting audio (track %d)...\n", streamIdx)
		samples, err := ExtractAudio(file, streamIdx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error extracting audio: %v\n", err)
			continue
		}

		// Track current output for signal cleanup.
		currentOutput = outPath

		// Transcribe with progress.
		durationSec := float64(len(samples)) / 16000.0
		fmt.Printf("  Transcribing %.0f min of audio...\n", durationSec/60.0)

		progress := NewProgressReporter()
		segments, err := wm.Transcribe(samples, *language, progress.Update)
		progress.Finish()

		if err != nil {
			currentOutput = ""
			fmt.Fprintf(os.Stderr, "  Error transcribing: %v\n", err)
			continue
		}

		if *language == "" {
			lang := wm.DetectedLanguage()
			if lang != "" {
				fmt.Printf("  Detected language: %s\n", lang)
			}
		}

		// Write subtitle file.
		f, err := os.Create(outPath)
		if err != nil {
			currentOutput = ""
			fmt.Fprintf(os.Stderr, "  Error creating output file: %v\n", err)
			continue
		}

		switch *format {
		case "vtt":
			err = WriteVTT(f, segments)
		default:
			err = WriteSRT(f, segments)
		}
		f.Close()
		currentOutput = ""

		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error writing subtitles: %v\n", err)
			os.Remove(outPath)
			continue
		}

		elapsed := time.Since(progress.startTime)
		em := int(elapsed.Seconds()) / 60
		es := int(elapsed.Seconds()) % 60
		fmt.Printf("  Done: %d segments in %dm%02ds -> %s\n\n", len(segments), em, es, outPath)
	}

	fmt.Println("All done.")
}

