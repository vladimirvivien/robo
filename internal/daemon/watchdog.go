package daemon

import (
	"context"
	"sync"
	"time"
)

// Watchdog tracks activity and triggers a shutdown callback when idle for longer than IdleTTL.
type Watchdog struct {
	idleTTL   time.Duration
	lastTouch time.Time
	onTimeout func()
	stopCh    chan struct{}
	mu        sync.Mutex
	running   bool
}

// NewWatchdog creates a new Watchdog with the specified idle TTL and timeout callback.
func NewWatchdog(idleTTL time.Duration, onTimeout func()) *Watchdog {
	if idleTTL <= 0 {
		idleTTL = 15 * time.Minute
	}
	return &Watchdog{
		idleTTL:   idleTTL,
		lastTouch: time.Now(),
		onTimeout: onTimeout,
		stopCh:    make(chan struct{}),
	}
}

// Touch records active use, resetting the idle countdown.
func (w *Watchdog) Touch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastTouch = time.Now()
}

// LastTouch returns the timestamp of the last recorded activity.
func (w *Watchdog) LastTouch() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastTouch
}

// Start begins the background check ticker.
func (w *Watchdog) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	checkInterval := max(w.idleTTL/5, 100*time.Millisecond)
	if checkInterval > 30*time.Second {
		checkInterval = 30 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			elapsed := time.Since(w.lastTouch)
			timedOut := elapsed > w.idleTTL
			w.mu.Unlock()

			if timedOut {
				if w.onTimeout != nil {
					w.onTimeout()
				}
				return
			}
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop terminates the watchdog loop.
func (w *Watchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
}
