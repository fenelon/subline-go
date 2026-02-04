package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Segment represents a single subtitle segment with start/end times and text.
type Segment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// FormatTimestamp converts a time.Duration into an SRT or VTT timestamp string.
//
// SRT format: HH:MM:SS,mmm  (comma separator)
// VTT format: HH:MM:SS.mmm  (dot separator)
//
// The format argument should be "srt" or "vtt".
func FormatTimestamp(d time.Duration, format string) string {
	total := d.Milliseconds()
	ms := total % 1000
	total /= 1000
	hours := total / 3600
	total %= 3600
	minutes := total / 60
	seconds := total % 60

	ts := fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
	if format == "srt" {
		ts = strings.Replace(ts, ".", ",", 1)
	}
	return ts
}

// WriteSRT writes segments in SRT (SubRip) format to w.
//
// SRT format:
//
//	1
//	00:00:00,000 --> 00:00:02,000
//	Hello world
//
//	2
//	...
func WriteSRT(w io.Writer, segments []Segment) error {
	for i, seg := range segments {
		start := FormatTimestamp(seg.Start, "srt")
		end := FormatTimestamp(seg.End, "srt")
		text := strings.TrimSpace(seg.Text)
		_, err := fmt.Fprintf(w, "%d\n%s --> %s\n%s\n\n", i+1, start, end, text)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteVTT writes segments in WebVTT format to w.
//
// VTT format:
//
//	WEBVTT
//
//	00:00:00.000 --> 00:00:02.000
//	Hello world
//
//	00:00:03.000 --> 00:00:05.000
//	...
func WriteVTT(w io.Writer, segments []Segment) error {
	if _, err := fmt.Fprint(w, "WEBVTT\n\n"); err != nil {
		return err
	}
	for _, seg := range segments {
		start := FormatTimestamp(seg.Start, "vtt")
		end := FormatTimestamp(seg.End, "vtt")
		text := strings.TrimSpace(seg.Text)
		_, err := fmt.Fprintf(w, "%s --> %s\n%s\n\n", start, end, text)
		if err != nil {
			return err
		}
	}
	return nil
}
