package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// FormatTimestamp tests
// ---------------------------------------------------------------------------

func TestFormatTimestamp_SRTZero(t *testing.T) {
	got := FormatTimestamp(0, "srt")
	want := "00:00:00,000"
	if got != want {
		t.Errorf("FormatTimestamp(0, srt) = %q; want %q", got, want)
	}
}

func TestFormatTimestamp_VTTZero(t *testing.T) {
	got := FormatTimestamp(0, "vtt")
	want := "00:00:00.000"
	if got != want {
		t.Errorf("FormatTimestamp(0, vtt) = %q; want %q", got, want)
	}
}

func TestFormatTimestamp_SRTUsesComma(t *testing.T) {
	d := time.Duration(1500 * float64(time.Millisecond)) // 1.5s
	got := FormatTimestamp(d, "srt")
	if strings.Contains(got, ".") {
		t.Errorf("SRT timestamp should use comma, not dot: %q", got)
	}
	if !strings.Contains(got, ",") {
		t.Errorf("SRT timestamp should contain comma: %q", got)
	}
}

func TestFormatTimestamp_VTTUsesDot(t *testing.T) {
	d := time.Duration(1500 * float64(time.Millisecond)) // 1.5s
	got := FormatTimestamp(d, "vtt")
	if strings.Contains(got, ",") {
		t.Errorf("VTT timestamp should use dot, not comma: %q", got)
	}
	if !strings.Contains(got, ".") {
		t.Errorf("VTT timestamp should contain dot: %q", got)
	}
}

func TestFormatTimestamp_LargeValueSRT(t *testing.T) {
	// 39599.999 seconds = 10h 59m 59.999s
	d := time.Duration(39599999 * float64(time.Millisecond))
	got := FormatTimestamp(d, "srt")
	want := "10:59:59,999"
	if got != want {
		t.Errorf("FormatTimestamp(39599.999s, srt) = %q; want %q", got, want)
	}
}

func TestFormatTimestamp_FractionalVTT(t *testing.T) {
	// 0.123 seconds
	d := 123 * time.Millisecond
	got := FormatTimestamp(d, "vtt")
	want := "00:00:00.123"
	if got != want {
		t.Errorf("FormatTimestamp(0.123s, vtt) = %q; want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// WriteSRT tests
// ---------------------------------------------------------------------------

func TestWriteSRT_NumberedEntries(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 2 * time.Second, Text: "Hello"},
		{Start: 3 * time.Second, End: 5 * time.Second, Text: "World"},
	}
	var buf bytes.Buffer
	err := WriteSRT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteSRT returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "1\n") {
		t.Error("WriteSRT should start first entry with 1")
	}
	if !strings.Contains(out, "2\n") {
		t.Error("WriteSRT should number second entry as 2")
	}
}

func TestWriteSRT_TimestampFormat(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 2 * time.Second, Text: "Hello"},
	}
	var buf bytes.Buffer
	err := WriteSRT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteSRT returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "00:00:00,000 --> 00:00:02,000") {
		t.Errorf("WriteSRT should use SRT timestamp format with commas, got:\n%s", out)
	}
}

func TestWriteSRT_TrimsWhitespace(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 1 * time.Second, Text: "  padded text  "},
	}
	var buf bytes.Buffer
	err := WriteSRT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteSRT returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "  padded") {
		t.Errorf("WriteSRT should trim leading whitespace from text, got:\n%s", out)
	}
	if strings.Contains(out, "text  ") {
		t.Errorf("WriteSRT should trim trailing whitespace from text, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// WriteVTT tests
// ---------------------------------------------------------------------------

func TestWriteVTT_Header(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 1 * time.Second, Text: "Hello"},
	}
	var buf bytes.Buffer
	err := WriteVTT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteVTT returned error: %v", err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, "WEBVTT\n\n") {
		t.Errorf("WriteVTT should start with WEBVTT header, got:\n%q", out[:min(len(out), 30)])
	}
}

func TestWriteVTT_TimestampFormat(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 2 * time.Second, Text: "Hello"},
	}
	var buf bytes.Buffer
	err := WriteVTT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteVTT returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "00:00:00.000 --> 00:00:02.000") {
		t.Errorf("WriteVTT should use VTT timestamp format with dots, got:\n%s", out)
	}
	if strings.Contains(out, ",") {
		t.Error("WriteVTT timestamps should not contain commas")
	}
}

func TestWriteVTT_NoIndexNumbers(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 1 * time.Second, Text: "First"},
		{Start: 2 * time.Second, End: 3 * time.Second, Text: "Second"},
	}
	var buf bytes.Buffer
	err := WriteVTT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteVTT returned error: %v", err)
	}
	out := buf.String()

	// After the WEBVTT header, lines should not start with a bare number
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip WEBVTT header
		}
		if line == "1" || line == "2" {
			t.Errorf("WriteVTT should not include index numbers, found %q at line %d", line, i+1)
		}
	}
}

func TestWriteVTT_TrimsWhitespace(t *testing.T) {
	segments := []Segment{
		{Start: 0, End: 1 * time.Second, Text: "  padded text  "},
	}
	var buf bytes.Buffer
	err := WriteVTT(&buf, segments)
	if err != nil {
		t.Fatalf("WriteVTT returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "  padded") {
		t.Errorf("WriteVTT should trim leading whitespace from text, got:\n%s", out)
	}
	if strings.Contains(out, "text  ") {
		t.Errorf("WriteVTT should trim trailing whitespace from text, got:\n%s", out)
	}
}
