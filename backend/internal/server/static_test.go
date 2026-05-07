package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644)

	appDir := filepath.Join(dir, "_app", "immutable")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "chunk-abc123.js"), []byte("console.log('hi')"), 0644)

	os.WriteFile(filepath.Join(dir, "favicon.png"), []byte("PNG"), 0644)

	os.WriteFile(filepath.Join(dir, "index.html.gz"), []byte("gzipped"), 0644)
	os.WriteFile(filepath.Join(dir, "index.html.br"), []byte("brotli"), 0644)

	return dir
}

func TestSPAFileServer_ServesExistingFile(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "PNG" {
		t.Fatalf("expected PNG body, got %q", rec.Body.String())
	}
}

func TestSPAFileServer_FallbackToIndex(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "<html>SPA</html>" {
		t.Fatalf("expected SPA index, got %q", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("index.html should have no-cache, got %q", cc)
	}
}

func TestSPAFileServer_HashedAssetImmutableCache(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/chunk-abc123.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache for _app assets, got %q", cc)
	}
}

func TestSPAFileServer_PreCompressedBrotli(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "br" {
		t.Fatalf("expected br encoding, got %q", ce)
	}
}

func TestSPAFileServer_PreCompressedGzip(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", ce)
	}
}

func TestSPAFileServer_DoesNotServeAPIFallback(t *testing.T) {
	dir := setupStaticDir(t)
	handler := NewSPAFileServer(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/ path, got %d", rec.Code)
	}
}
