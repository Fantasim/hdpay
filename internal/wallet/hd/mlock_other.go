//go:build !linux

package hd

import "log/slog"

// MlockBytes is a no-op on non-Linux platforms.
// On Linux, this pins memory pages in RAM to prevent swapping secrets to disk.
func MlockBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	slog.Debug("mlock not supported on this platform", "len", len(b))
}

// MunlockBytes is a no-op on non-Linux platforms.
func MunlockBytes(b []byte) {}
