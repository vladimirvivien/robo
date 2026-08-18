package daemon_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/daemon"
)

func TestWatchdog_Timeout(t *testing.T) {
	var timedOut atomic.Bool

	ttl := 100 * time.Millisecond
	wd := daemon.NewWatchdog(ttl, func() {
		timedOut.Store(true)
	})

	ctx := t.Context()

	go wd.Start(ctx)

	// Wait longer than TTL
	time.Sleep(250 * time.Millisecond)

	if !timedOut.Load() {
		t.Fatal("expected watchdog timeout to trigger callback")
	}
}

func TestWatchdog_TouchResetsTimer(t *testing.T) {
	var timedOut atomic.Bool

	ttl := 200 * time.Millisecond
	wd := daemon.NewWatchdog(ttl, func() {
		timedOut.Store(true)
	})

	ctx := t.Context()

	go wd.Start(ctx)

	// Touch periodically before TTL expires
	for i := range 3 {
		time.Sleep(100 * time.Millisecond)
		wd.Touch()
		if timedOut.Load() {
			t.Fatalf("watchdog timed out prematurely on touch cycle %d", i)
		}
	}

	wd.Stop()
}
