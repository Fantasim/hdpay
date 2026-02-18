package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler_ServesStaticFile(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html":                  {Data: []byte("<html>SPA</html>")},
		"robots.txt":                  {Data: []byte("User-agent: *")},
		"_app/immutable/test.js":      {Data: []byte("console.log('test')")},
	}

	handler := SPAHandler(staticFS)

	// Serve robots.txt directly.
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for robots.txt", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache for robots.txt", w.Header().Get("Cache-Control"))
	}
}

func TestSPAHandler_ImmutableCacheHeaders(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html":                  {Data: []byte("<html>SPA</html>")},
		"_app/immutable/chunk.js":     {Data: []byte("chunk")},
	}

	handler := SPAHandler(staticFS)

	req := httptest.NewRequest("GET", "/_app/immutable/chunk.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable header", w.Header().Get("Cache-Control"))
	}
}

func TestSPAHandler_FallbackToIndex(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>SPA</html>")},
	}

	handler := SPAHandler(staticFS)

	// Unknown path should fallback to index.html.
	req := httptest.NewRequest("GET", "/settings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for SPA fallback", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "<html>SPA</html>" {
		t.Errorf("body = %q, want SPA index", w.Body.String())
	}
}

func TestSPAHandler_APIPathReturns404(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>SPA</html>")},
	}

	handler := SPAHandler(staticFS)

	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for /api/* paths", w.Code)
	}
}

func TestSPAHandler_RootServesIndex(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>Home</html>")},
	}

	handler := SPAHandler(staticFS)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for root", w.Code)
	}
}
