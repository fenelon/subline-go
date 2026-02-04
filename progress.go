package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ProgressReporter displays transcription progress to stderr.
type ProgressReporter struct {
	startTime  time.Time
	segments   int
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewProgressReporter creates a ProgressReporter and records the start time.
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		startTime: time.Now(),
	}
}

// Update is called by the transcription loop after each segment.
// It throttles output to at most once per second using \r for in-place updates.
func (p *ProgressReporter) Update(pct int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if now.Sub(p.lastUpdate) < time.Second {
		return
	}
	p.lastUpdate = now

	elapsed := now.Sub(p.startTime).Seconds()
	if pct > 0 && pct < 100 {
		eta := elapsed / float64(pct) * float64(100-pct)
		etaMin := int(eta) / 60
		etaSec := int(eta) % 60
		fmt.Fprintf(os.Stderr, "\r  [%5.1f%%] %0.fs elapsed, ETA %dm%02ds   ", float64(pct), elapsed, etaMin, etaSec)
	} else {
		fmt.Fprintf(os.Stderr, "\r  [%5.1f%%] %.0fs elapsed   ", float64(pct), elapsed)
	}
}

// Finish prints a final newline to clear the \r progress line.
func (p *ProgressReporter) Finish() {
	fmt.Fprintf(os.Stderr, "\r%80s\r", "") // clear the line
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
