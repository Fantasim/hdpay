//go:build linux

package hd

import (
	"log/slog"
	"syscall"
)

// MlockBytes pins a byte slice's memory pages in RAM, preventing the OS from
// swapping them to disk. Non-fatal: logs a warning if it fails (may require
// CAP_IPC_LOCK or elevated privileges).
func MlockBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	if err := syscall.Mlock(b); err != nil {
		slog.Warn("mlock failed (secrets may be swappable)", "error", err, "len", len(b))
	}
}

// MunlockBytes unlocks previously mlocked memory pages. Should be called
// before zeroing and releasing the slice.
func MunlockBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	if err := syscall.Munlock(b); err != nil {
		slog.Warn("munlock failed", "error", err, "len", len(b))
	}
}
