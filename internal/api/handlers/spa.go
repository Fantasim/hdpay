package handlers

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

// SPAHandler serves the embedded SvelteKit static build with SPA fallback.
// For any path that doesn't match a real file, it serves index.html so that
// client-side routing can handle the path.
func SPAHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip API routes (should never reach here due to router ordering, but safety check).
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to open the file to check if it exists.
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		f, err := staticFS.Open(cleanPath)
		if err == nil {
			f.Close()

			// Set cache headers: immutable for _app/ hashed assets, no-cache for everything else.
			if strings.HasPrefix(path, "/_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}

			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA client-side routing.
		slog.Debug("SPA fallback", "path", path)

		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		indexFile, err := staticFS.Open("index.html")
		if err != nil {
			slog.Error("failed to open SPA index.html", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer indexFile.Close()

		stat, err := indexFile.Stat()
		if err != nil {
			slog.Error("failed to stat SPA index.html", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(readSeeker))
	}
}

// readSeeker combines io.ReadSeeker for http.ServeContent.
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}
