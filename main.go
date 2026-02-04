package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	// Banner.
	fmt.Printf("\n\033[1m"+
		"░▄▀▀░█▒█░██▄░█▒░░█░█▄░█▒██▀\n"+
		"▒▄██░▀▄█▒█▄█▒█▄▄░█░█▒▀█░█▄▄\n"+
		"\033[0m\n"+
		"Subline %s - AI subtitles made easy\n\n", Version)

	// Parse flags (with shorthands).
	var language, model, format, outputDir string
	var audioTrack int
	var skipExisting, verbose bool

	flag.StringVar(&language, "language", "", "Language code (auto-detect if omitted)")
	flag.StringVar(&language, "l", "", "Language code (shorthand)")
	flag.StringVar(&model, "model", "turbo", "Whisper model (tiny/base/small/medium/turbo/large)")
	flag.StringVar(&model, "m", "turbo", "Whisper model (shorthand)")
	flag.IntVar(&audioTrack, "audio-track", -1, "Audio stream index (-1 = auto-detect)")
	flag.IntVar(&audioTrack, "a", -1, "Audio stream index (shorthand)")
	flag.StringVar(&format, "format", "srt", "Output format: srt or vtt")
	flag.StringVar(&format, "f", "srt", "Output format (shorthand)")
	flag.StringVar(&outputDir, "output-dir", "", "Directory to write subtitle files (default: next to source)")
	flag.StringVar(&outputDir, "o", "", "Output directory (shorthand)")
	flag.BoolVar(&skipExisting, "skip-existing", false, "Skip files that already have a subtitle file")
	flag.BoolVar(&skipExisting, "s", false, "Skip existing (shorthand)")
	flag.BoolVar(&verbose, "verbose", false, "Show detailed model loading and engine output")
	flag.BoolVar(&verbose, "v", false, "Verbose (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: subline [options] <path...>\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -l, --language string    Language code (auto-detect if omitted)\n")
		fmt.Fprintf(os.Stderr, "  -m, --model string       Whisper model (tiny/base/small/medium/turbo/large) (default \"turbo\")\n")
		fmt.Fprintf(os.Stderr, "  -a, --audio-track int    Audio stream index (-1 = auto-detect) (default -1)\n")
		fmt.Fprintf(os.Stderr, "  -f, --format string      Output format: srt or vtt (default \"srt\")\n")
		fmt.Fprintf(os.Stderr, "  -o, --output-dir string  Directory to write subtitle files (default: next to source)\n")
		fmt.Fprintf(os.Stderr, "  -s, --skip-existing      Skip files that already have a subtitle file\n")
		fmt.Fprintf(os.Stderr, "  -v, --verbose            Show detailed model loading and engine output\n")
		fmt.Fprintln(os.Stderr)
	}
	flag.Parse()

	if format != "srt" && format != "vtt" {
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

	langStr := language
	if langStr == "" {
		langStr = "auto-detect"
	}
	fmt.Printf("Found %d file(s) | model=%s | language=%s | device=%s\n\n", len(files), model, langStr, device)

	// Ensure model is downloaded.
	fmt.Fprintf(os.Stderr, "Loading model '%s'...\n", model)
	modelPath, err := EnsureModel(model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Helper to run a function with C stdout/stderr suppressed.
	quiet := func(fn func()) {
		if verbose {
			fn()
			return
		}
		restore := suppressCOutput()
		defer restore()
		fn()
	}

	// Load whisper model (suppress C library logging unless --verbose).
	var wm *WhisperModel
	quiet(func() { wm, err = LoadModel(modelPath) })
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		os.Exit(1)
	}
	defer func() { quiet(func() { wm.Close() }) }()
	fmt.Println()

	// Create output directory if specified.
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
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

		// Probe audio tracks.
		tracks, err := ProbeAudioTracks(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error probing audio: %v\n", err)
			continue
		}

		// Pick audio track(s).
		streamIndices, err := PickAudioTracks(tracks, audioTrack)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %v, skipping\n", err)
			continue
		}

		multiTrack := len(streamIndices) > 1

		for _, streamIdx := range streamIndices {
			// Determine output path.
			base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			if multiTrack {
				lang := TrackLanguage(tracks, streamIdx)
				base = base + "." + lang
			}
			outName := base + "." + format
			var outPath string
			if outputDir != "" {
				outPath = filepath.Join(outputDir, outName)
			} else {
				outPath = filepath.Join(filepath.Dir(file), outName)
			}

			// Skip existing.
			if skipExisting {
				if _, err := os.Stat(outPath); err == nil {
					fmt.Println("  Skipping (subtitle file exists)")
					continue
				}
			}

			// Extract audio.
			fmt.Printf("  Extracting audio (track %d)...\n", streamIdx)
			var samples []float32
			quiet(func() { samples, err = ExtractAudio(file, streamIdx) })
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error extracting audio: %v\n", err)
				continue
			}

			// Track current output for signal cleanup.
			currentOutput = outPath

			// Detect language before transcription if not specified.
			transcribeLang := language
			if language == "" {
				var detected string
				quiet(func() { detected = wm.DetectLanguage(samples) })
				if detected != "" {
					fmt.Printf("  Detected language: %s\n", detected)
					transcribeLang = detected
				}
			}

			// Transcribe with progress.
			durationSec := float64(len(samples)) / 16000.0
			if durationSec < 60 {
				fmt.Printf("  Transcribing %.0fs of audio...\n", durationSec)
			} else {
				fmt.Printf("  Transcribing %.0f min of audio...\n", durationSec/60.0)
			}

			progress := NewProgressReporter(realStderr)
			var segments []Segment
			quiet(func() { segments, err = wm.Transcribe(samples, transcribeLang, progress.Update) })
			progress.Finish()

			if err != nil {
				currentOutput = ""
				fmt.Fprintf(os.Stderr, "  Error transcribing: %v\n", err)
				continue
			}

			// Write subtitle file.
			f, err := os.Create(outPath)
			if err != nil {
				currentOutput = ""
				fmt.Fprintf(os.Stderr, "  Error creating output file: %v\n", err)
				continue
			}

			switch format {
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
	}

	fmt.Println("All done.")
}

// realStderr is a dup of the original stderr fd, unaffected by suppressCOutput.
// The progress bar writes to this so it remains visible even when C output is muted.
var realStderr *os.File

func init() {
	fd, err := syscall.Dup(2)
	if err == nil {
		realStderr = os.NewFile(uintptr(fd), "realStderr")
	} else {
		realStderr = os.Stderr
	}
}

// suppressCOutput redirects C-level stdout and stderr to /dev/null.
// Call the returned function to restore them.
func suppressCOutput() func() {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return func() {}
	}

	// Save original file descriptors.
	savedStdout, err1 := syscall.Dup(1)
	savedStderr, err2 := syscall.Dup(2)
	if err1 != nil || err2 != nil {
		devNull.Close()
		return func() {}
	}

	// Redirect stdout and stderr to /dev/null.
	syscall.Dup2(int(devNull.Fd()), 1)
	syscall.Dup2(int(devNull.Fd()), 2)
	devNull.Close()

	return func() {
		syscall.Dup2(savedStdout, 1)
		syscall.Dup2(savedStderr, 2)
		syscall.Close(savedStdout)
		syscall.Close(savedStderr)
	}
}
