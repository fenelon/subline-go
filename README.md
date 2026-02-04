# Subline

Generate subtitles for any video or audio file using [whisper.cpp](https://github.com/ggerganov/whisper.cpp), with Metal GPU acceleration on macOS.

<img width="912" height="744" alt="SCR-20260204-tzho" src="https://github.com/user-attachments/assets/e8fdf01e-496a-4e31-9df7-e0d29827e3ae" />

## Features

- Automatic language detection (99 languages supported)
- Metal GPU acceleration on macOS, CPU on Linux
- Models auto-download from HuggingFace on first use
- SRT and VTT output formats
- Multi-track audio support with interactive track selection
- Batch processing of files and directories
- Progress bar with ETA
- Skips already-subtitled files with `--skip-existing`

## Supported formats

**Video:** mp4, mkv, avi, mov, webm
**Audio:** wav, flac, mp3
**Codecs:** AAC, MP3, FLAC, Opus, Vorbis, AC3, E-AC3, DTS, PCM

## Usage

```
subline [options] <path...>
```

Paths can be individual files or directories (all media files inside will be processed).

### Options

| Flag | Values | Description | Default |
|------|--------|-------------|---------|
| `-l, --language` | `en`, `ru`, `de`, `fr`, ... ([ISO 639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)) | Language code | auto-detect |
| `-m, --model` | `tiny`, `base`, `small`, `medium`, `turbo`, `large` | Whisper model | `turbo` |
| `-a, --audio-track` | `0`, `1`, `2`, ... | Audio stream index | auto-detect |
| `-f, --format` | `srt`, `vtt` | Output subtitle format | `srt` |
| `-o, --output-dir` | path | Directory for subtitle files | next to source |
| `-s, --skip-existing` | | Skip files that already have subtitles | |
| `-v, --verbose` | | Show whisper.cpp engine output | |

### Examples

```bash
# Subtitles for a single file (auto-detect language)
subline movie.mp4

# Specify language and model
subline -l ru -m large movie.mkv

# Process a whole directory, output as VTT
subline -f vtt -o ./subs/ ~/Movies/

# Skip already processed files
subline -s ~/Movies/
```

## Models

Models are downloaded automatically to `~/Library/Caches/subline/models` (macOS) or `~/.cache/subline/models` (Linux).

| Model | Size | Notes |
|-------|------|-------|
| `tiny` | 75 MB | Fastest, least accurate |
| `base` | 142 MB | |
| `small` | 466 MB | |
| `medium` | 1.5 GB | |
| `turbo` | 1.6 GB | Best speed/quality tradeoff (default) |
| `large` | 3.1 GB | Most accurate |

## Build

Requires: Go 1.22+, CMake, a C/C++ compiler. FFmpeg and whisper.cpp are built from vendored sources.

```bash
git clone --recursive https://github.com/fenelon/subline-go
cd subline-go
make
```

The binary is output to `./subline`.
