//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package ui

// RestoreCookedMode is a no-op on other systems.
func RestoreCookedMode() {}
