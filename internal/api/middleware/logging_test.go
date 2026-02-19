package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestResponseWriter_ImplementsFlusher verifies that the logging middleware's
// responseWriter implements http.Flusher, which is required for SSE streaming.
func TestResponseWriter_ImplementsFlusher(t *testing.T) {
	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("responseWriter does not implement http.Flusher — SSE streaming will break")
		}
		// Should not panic.
		flusher.Flush()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", w.Body.String())
	}
}

// TestResponseWriter_Unwrap verifies the Unwrap method for interface assertion passthrough.
func TestResponseWriter_Unwrap(t *testing.T) {
	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw, ok := w.(*responseWriter)
		if !ok {
			t.Fatal("expected *responseWriter")
		}
		unwrapped := rw.Unwrap()
		if unwrapped == nil {
			t.Fatal("Unwrap() returned nil")
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

// TestResponseWriter_CapturesStatusAndSize verifies that status and size are tracked.
func TestResponseWriter_CapturesStatusAndSize(t *testing.T) {
	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("POST", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", w.Body.String())
	}
}
