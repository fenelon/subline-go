package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const barWidth = 30

// ProgressReporter displays a progress bar during transcription.
type ProgressReporter struct {
	startTime  time.Time
	lastUpdate time.Time
	w          io.Writer
	mu         sync.Mutex
}

// NewProgressReporter creates a ProgressReporter that writes to w.
func NewProgressReporter(w io.Writer) *ProgressReporter {
	return &ProgressReporter{
		startTime: time.Now(),
		w:         w,
	}
}

// Update renders a progress bar. Called by the transcription engine with pct in [0,100].
// Throttled to at most once per 200ms.
func (p *ProgressReporter) Update(pct int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if now.Sub(p.lastUpdate) < 200*time.Millisecond {
		return
	}
	p.lastUpdate = now

	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	filled := barWidth * pct / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	elapsed := now.Sub(p.startTime).Seconds()
	elapsedStr := formatDuration(elapsed)

	if pct > 0 && pct < 100 {
		eta := elapsed / float64(pct) * float64(100-pct)
		etaStr := formatDuration(eta)
		fmt.Fprintf(p.w, "\r  %s %3d%% | %s elapsed | ETA %s  ", bar, pct, elapsedStr, etaStr)
	} else {
		fmt.Fprintf(p.w, "\r  %s %3d%% | %s elapsed  ", bar, pct, elapsedStr)
	}
}

// Finish clears the progress bar line.
func (p *ProgressReporter) Finish() {
	fmt.Fprintf(p.w, "\r%80s\r", "")
}

// formatDuration formats seconds into a compact string like "1m23s" or "5s".
func formatDuration(secs float64) string {
	s := int(secs)
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s = s % 60
	if m < 60 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
}

// SignalCleanup registers a handler for SIGINT that removes the given file
// (if non-empty) before exiting. Call the returned function to deregister
// the handler and stop the goroutine.
func SignalCleanup(partialPath *string) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			if partialPath != nil && *partialPath != "" {
				os.Remove(*partialPath)
				fmt.Fprintf(os.Stderr, "\nInterrupted, removed partial file: %s\n", *partialPath)
			} else {
				fmt.Fprintf(os.Stderr, "\nInterrupted.\n")
			}
			os.Exit(1)
		case <-done:
		}
	}()

	return func() {
		signal.Stop(ch)
		close(done)
	}
}
