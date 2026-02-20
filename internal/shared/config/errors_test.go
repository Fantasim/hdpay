package config

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTransientError_Wrap(t *testing.T) {
	original := errors.New("connection refused")
	wrapped := NewTransientError(original)

	if wrapped.Error() != "connection refused" {
		t.Errorf("expected 'connection refused', got %q", wrapped.Error())
	}

	// Unwrap should return original
	unwrapped := errors.Unwrap(wrapped)
	if unwrapped != original {
		t.Errorf("expected original error, got %v", unwrapped)
	}
}

func TestTransientError_IsTransient(t *testing.T) {
	transient := NewTransientError(errors.New("timeout"))
	if !IsTransient(transient) {
		t.Error("expected IsTransient() = true for transient error")
	}

	// Wrapped in fmt.Errorf should still be detectable
	wrapped := fmt.Errorf("provider failed: %w", transient)
	if !IsTransient(wrapped) {
		t.Error("expected IsTransient() = true for wrapped transient error")
	}
}

func TestTransientError_WithRetryAfter(t *testing.T) {
	err := NewTransientErrorWithRetry(errors.New("rate limited"), 5*time.Second)

	if !IsTransient(err) {
		t.Error("expected IsTransient() = true")
	}

	retryAfter := GetRetryAfter(err)
	if retryAfter != 5*time.Second {
		t.Errorf("expected retry after 5s, got %v", retryAfter)
	}

	// Wrapped
	wrapped := fmt.Errorf("outer: %w", err)
	retryAfter = GetRetryAfter(wrapped)
	if retryAfter != 5*time.Second {
		t.Errorf("expected retry after 5s for wrapped, got %v", retryAfter)
	}
}

func TestPermanentError_NotTransient(t *testing.T) {
	permanent := errors.New("invalid mnemonic")

	if IsTransient(permanent) {
		t.Error("expected IsTransient() = false for permanent error")
	}

	if GetRetryAfter(permanent) != 0 {
		t.Error("expected GetRetryAfter() = 0 for permanent error")
	}

	// nil error
	if IsTransient(nil) {
		t.Error("expected IsTransient(nil) = false")
	}
	if GetRetryAfter(nil) != 0 {
		t.Error("expected GetRetryAfter(nil) = 0")
	}
}
