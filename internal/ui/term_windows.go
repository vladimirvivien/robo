//go:build windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableExtendedFlags  = 0x0080
	enableQuickEditMode  = 0x0040
	enableAutoPosition   = 0x0100
)

// RestoreCookedMode explicitly restores Windows console input to standard interactive echo & line mode.
func RestoreCookedMode() {
	hIn := syscall.Handle(os.Stdin.Fd())
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(uintptr(hIn), uintptr(unsafe.Pointer(&mode)))
	if r != 0 {
		// Standard Cooked Mode: Processed + Line Input + Echo + Extended Flags + Quick Edit
		cookedMode := enableProcessedInput | enableLineInput | enableEchoInput | enableExtendedFlags | enableQuickEditMode | enableAutoPosition
		_, _, _ = procSetConsoleMode.Call(uintptr(hIn), uintptr(cookedMode))
	}
}
