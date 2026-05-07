package server

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// spaFileServer serves a SvelteKit static build with SPA fallback behavior.
type spaFileServer struct {
	dir string
}

// NewSPAFileServer returns an http.Handler that serves static files from dir.
//
// Behavior:
//   - Existing files are served directly.
//   - Paths starting with /api/ that have no matching file receive 404.
//   - All other missing paths fall back to index.html (SPA client-side routing).
//   - SvelteKit hashed assets under /_app/ receive an immutable cache header.
//   - index.html and SPA fallback responses receive Cache-Control: no-cache.
//   - Pre-compressed .br and .gz variants are served when the client accepts them
//     (brotli takes priority over gzip).
//   - Content-Type is derived from the original file extension.
//   - Paths containing ".." are rejected with 400.
func NewSPAFileServer(dir string) http.Handler {
	return &spaFileServer{dir: dir}
}

func (s *spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	// Directory traversal guard.
	if strings.Contains(urlPath, "..") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Clean the path and resolve it inside the root directory.
	cleaned := filepath.Clean("/" + strings.TrimPrefix(urlPath, "/"))
	fullPath := filepath.Join(s.dir, cleaned)

	info, err := os.Stat(fullPath)
	fileExists := err == nil && !info.IsDir()

	if fileExists {
		s.serveFile(w, r, fullPath, cleaned)
		return
	}

	// File does not exist — check whether the path is under /api/.
	if strings.HasPrefix(urlPath, "/api/") || urlPath == "/api" {
		http.NotFound(w, r)
		return
	}

	// SPA fallback: serve index.html for all other paths.
	indexPath := filepath.Join(s.dir, "index.html")
	w.Header().Set("Cache-Control", "no-cache")
	s.serveFile(w, r, indexPath, "/index.html")
}

// serveFile writes a single file to the response, applying pre-compression and
// cache headers appropriate for the logical path.
func (s *spaFileServer) serveFile(w http.ResponseWriter, r *http.Request, diskPath, logicalPath string) {
	// Determine Content-Type from the original extension before any compression suffix.
	ext := filepath.Ext(logicalPath)
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	// Apply cache policy based on the logical path.
	if strings.HasPrefix(logicalPath, "/_app/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if logicalPath == "/index.html" {
		// Only set when not already set by the fallback path in ServeHTTP.
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "no-cache")
		}
	}

	// Attempt to serve a pre-compressed variant.
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if strings.Contains(acceptEncoding, "br") {
		if served := s.tryCompressed(w, diskPath, "br", diskPath+".br"); served {
			return
		}
	}
	if strings.Contains(acceptEncoding, "gzip") {
		if served := s.tryCompressed(w, diskPath, "gzip", diskPath+".gz"); served {
			return
		}
	}

	http.ServeFile(w, &http.Request{Method: http.MethodGet, URL: r.URL}, diskPath)
}

// tryCompressed attempts to serve a pre-compressed file. It returns true when
// the compressed variant exists and was written to w, false otherwise.
func (s *spaFileServer) tryCompressed(w http.ResponseWriter, originalPath, encoding, compressedPath string) bool {
	f, err := os.Open(compressedPath)
	if err != nil {
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}

	w.Header().Set("Content-Encoding", encoding)
	w.Header().Set("Vary", "Accept-Encoding")
	w.WriteHeader(http.StatusOK)

	buf := make([]byte, info.Size())
	if _, err := f.Read(buf); err == nil {
		w.Write(buf) //nolint:errcheck // best-effort write after header committed
	}
	return true
}
