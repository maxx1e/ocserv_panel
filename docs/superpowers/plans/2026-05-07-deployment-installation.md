# Deployment & Installation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable zero-dependency production deployment of ocserv-panel with native TLS, static file serving, release packaging, and an interactive installer.

**Architecture:** Go backend gains a static file server (SPA fallback) and native TLS with automatic cert reloading. SvelteKit switches to adapter-static (pure HTML/JS/CSS). A Makefile produces a release tarball. An interactive shell script handles install/upgrade on the target server.

**Tech Stack:** Go stdlib (net/http, crypto/tls), @sveltejs/adapter-static, GNU Make, POSIX shell (bash)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `backend/internal/config/config.go` | Modify | Add `StaticDir` field, update `Defaults()`, extend `MergeEnv()` |
| `backend/internal/config/config_test.go` | Modify | Test new fields and env merging |
| `backend/internal/server/static.go` | Create | SPA file server with pre-compression and cache headers |
| `backend/internal/server/static_test.go` | Create | Tests for SPA fallback, cache headers, pre-compression |
| `backend/internal/server/tls.go` | Create | Cert reloader with mtime watcher |
| `backend/internal/server/tls_test.go` | Create | Tests for cert loading and reload |
| `backend/internal/server/server.go` | Modify | Accept static dir param in `NewAppRouter` |
| `backend/cmd/ocserv-panel/main.go` | Modify | Wire TLS, static dir, version flag |
| `frontend/svelte.config.js` | Modify | Switch to adapter-static |
| `frontend/package.json` | Modify | Replace adapter-auto with adapter-static |
| `frontend/src/routes/+layout.ts` | Create | Export `prerender` and `ssr` settings for SPA mode |
| `Makefile` | Create | Build pipeline (backend, frontend, release) |
| `deploy/install.sh` | Create | Interactive installer script |
| `deploy/ocserv-panel.service` | Modify | Updated systemd unit with TLS/static env |
| `deploy/sudoers.d/ocserv-panel` | Modify | Full command allowlist |
| `deploy/config.toml.example` | Create | Reference configuration |

---

### Task 1: Add StaticDir to Config

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: Write test for StaticDir default and env merge**

Add to `backend/internal/config/config_test.go`:

```go
func TestDefaultsIncludeStaticDir(t *testing.T) {
	cfg := Defaults()
	if cfg.Installation.StaticDir != "/usr/share/ocserv-panel/static" {
		t.Fatalf("expected default StaticDir=/usr/share/ocserv-panel/static, got %q", cfg.Installation.StaticDir)
	}
}

func TestMergeEnvStaticDir(t *testing.T) {
	cfg := Defaults()
	MergeEnv(&cfg, func(key string) string {
		if key == "OCSERV_PANEL_STATIC_DIR" {
			return "/opt/custom/static"
		}
		return ""
	})
	if cfg.Installation.StaticDir != "/opt/custom/static" {
		t.Fatalf("expected StaticDir=/opt/custom/static after merge, got %q", cfg.Installation.StaticDir)
	}
}

func TestMergeEnvTLSCert(t *testing.T) {
	cfg := Defaults()
	MergeEnv(&cfg, func(key string) string {
		switch key {
		case "OCSERV_PANEL_TLS_CERT":
			return "/etc/letsencrypt/live/example.com/fullchain.pem"
		case "OCSERV_PANEL_TLS_KEY":
			return "/etc/letsencrypt/live/example.com/privkey.pem"
		}
		return ""
	})
	if cfg.Security.TLSCertPath != "/etc/letsencrypt/live/example.com/fullchain.pem" {
		t.Fatalf("TLSCertPath not merged from env")
	}
	if cfg.Security.TLSKeyPath != "/etc/letsencrypt/live/example.com/privkey.pem" {
		t.Fatalf("TLSKeyPath not merged from env")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/config/ -run "TestDefaultsIncludeStaticDir|TestMergeEnvStaticDir|TestMergeEnvTLSCert" -v`

Expected: FAIL — `StaticDir` field not found, `MergeEnv` doesn't handle new env vars.

- [ ] **Step 3: Add StaticDir field and update Defaults + MergeEnv**

In `backend/internal/config/config.go`, add to `InstallationConfig`:

```go
StaticDir string `toml:"static_dir"`
```

In `Defaults()`, add to the `Installation` block:

```go
StaticDir: "/usr/share/ocserv-panel/static",
```

In `MergeEnv()`, add at the end of the function:

```go
if v := getenv("OCSERV_PANEL_STATIC_DIR"); v != "" {
    cfg.Installation.StaticDir = v
}
if v := getenv("OCSERV_PANEL_TLS_CERT"); v != "" {
    cfg.Security.TLSCertPath = v
}
if v := getenv("OCSERV_PANEL_TLS_KEY"); v != "" {
    cfg.Security.TLSKeyPath = v
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/config/ -run "TestDefaultsIncludeStaticDir|TestMergeEnvStaticDir|TestMergeEnvTLSCert" -v`

Expected: PASS

- [ ] **Step 5: Run full config test suite to check no regressions**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/config/ -v`

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat(config): add StaticDir field and TLS env var merging"
```

---

### Task 2: SPA Static File Server

**Files:**
- Create: `backend/internal/server/static.go`
- Create: `backend/internal/server/static_test.go`

- [ ] **Step 1: Write tests for the SPA file server**

Create `backend/internal/server/static_test.go`:

```go
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

	// index.html
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644)

	// _app/immutable/chunk.js (hashed asset)
	appDir := filepath.Join(dir, "_app", "immutable")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "chunk-abc123.js"), []byte("console.log('hi')"), 0644)

	// favicon.png
	os.WriteFile(filepath.Join(dir, "favicon.png"), []byte("PNG"), 0644)

	// Pre-compressed file
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

	// /api/* requests should NOT get SPA fallback — they should 404
	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/ path, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestSPAFileServer" -v`

Expected: FAIL — `NewSPAFileServer` undefined.

- [ ] **Step 3: Implement SPA file server**

Create `backend/internal/server/static.go`:

```go
package server

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NewSPAFileServer returns an http.Handler that serves static files from dir
// with SPA fallback behavior: if a file doesn't exist and the path doesn't
// start with /api/, serve index.html instead.
//
// Cache policy:
//   - /_app/* paths: immutable (hashed filenames)
//   - index.html (and SPA fallback): no-cache
//   - Other static files: default (ETag-based)
//
// Pre-compression: if Accept-Encoding includes "br" or "gzip" and a
// corresponding .br or .gz file exists alongside the original, serve it
// with the appropriate Content-Encoding header.
func NewSPAFileServer(dir string) http.Handler {
	return &spaHandler{dir: dir}
}

type spaHandler struct {
	dir string
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path
	urlPath := r.URL.Path
	if urlPath == "" {
		urlPath = "/"
	}

	// Reject directory traversal
	cleanPath := filepath.Clean(urlPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Resolve file path
	filePath := filepath.Join(h.dir, filepath.FromSlash(cleanPath))

	// Check if the file exists
	info, err := os.Stat(filePath)
	if err == nil && !info.IsDir() {
		h.serveFile(w, r, filePath, cleanPath)
		return
	}

	// If it's a directory, try index.html in that directory
	if err == nil && info.IsDir() {
		indexPath := filepath.Join(filePath, "index.html")
		if _, indexErr := os.Stat(indexPath); indexErr == nil {
			h.serveFile(w, r, indexPath, cleanPath+"/index.html")
			return
		}
	}

	// SPA fallback: serve index.html for non-API paths
	if strings.HasPrefix(urlPath, "/api/") || strings.HasPrefix(urlPath, "/api") {
		http.NotFound(w, r)
		return
	}

	indexPath := filepath.Join(h.dir, "index.html")
	if _, indexErr := os.Stat(indexPath); indexErr != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	h.servePreCompressed(w, r, indexPath, "text/html; charset=utf-8")
}

func (h *spaHandler) serveFile(w http.ResponseWriter, r *http.Request, filePath, urlPath string) {
	// Determine content type
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set cache headers based on path
	if strings.HasPrefix(urlPath, "/_app/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if filepath.Base(filePath) == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	}

	h.servePreCompressed(w, r, filePath, contentType)
}

func (h *spaHandler) servePreCompressed(w http.ResponseWriter, r *http.Request, filePath, contentType string) {
	ae := r.Header.Get("Accept-Encoding")

	// Try brotli first
	if strings.Contains(ae, "br") {
		brPath := filePath + ".br"
		if _, err := os.Stat(brPath); err == nil {
			w.Header().Set("Content-Encoding", "br")
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Vary", "Accept-Encoding")
			data, _ := os.ReadFile(brPath)
			w.Write(data)
			return
		}
	}

	// Try gzip
	if strings.Contains(ae, "gzip") {
		gzPath := filePath + ".gz"
		if _, err := os.Stat(gzPath); err == nil {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Vary", "Accept-Encoding")
			data, _ := os.ReadFile(gzPath)
			w.Write(data)
			return
		}
	}

	// Serve original file
	w.Header().Set("Content-Type", contentType)
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestSPAFileServer" -v`

Expected: All 6 tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/static.go backend/internal/server/static_test.go
git commit -m "feat(server): add SPA file server with pre-compression and cache headers"
```

---

### Task 3: TLS Certificate Reloader

**Files:**
- Create: `backend/internal/server/tls.go`
- Create: `backend/internal/server/tls_test.go`

- [ ] **Step 1: Write tests for the cert reloader**

Create `backend/internal/server/tls_test.go`:

```go
package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func generateSelfSignedCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	os.WriteFile(certFile, certPEM, 0644)

	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	os.WriteFile(keyFile, keyPEM, 0600)

	return certFile, keyFile
}

func TestCertReloader_LoadsInitialCert(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSignedCert(t, dir)

	reloader, err := NewCertReloader(certFile, keyFile)
	if err != nil {
		t.Fatalf("NewCertReloader failed: %v", err)
	}

	hello := &tls.ClientHelloInfo{ServerName: "test"}
	cert, err := reloader.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil certificate")
	}
}

func TestCertReloader_ReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSignedCert(t, dir)

	reloader, err := NewCertReloader(certFile, keyFile)
	if err != nil {
		t.Fatalf("NewCertReloader failed: %v", err)
	}

	hello := &tls.ClientHelloInfo{ServerName: "test"}
	cert1, _ := reloader.GetCertificate(hello)

	// Regenerate the cert (different serial)
	time.Sleep(10 * time.Millisecond) // ensure mtime differs
	generateSelfSignedCert(t, dir)

	// Force reload check
	reloader.checkAndReload()

	cert2, _ := reloader.GetCertificate(hello)
	if cert1 == cert2 {
		t.Fatal("expected certificate to be reloaded after file change")
	}
}

func TestCertReloader_ErrorOnMissingFiles(t *testing.T) {
	_, err := NewCertReloader("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestCertReloader" -v`

Expected: FAIL — `NewCertReloader` undefined.

- [ ] **Step 3: Implement cert reloader**

Create `backend/internal/server/tls.go`:

```go
package server

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CertReloader manages a TLS certificate that can be reloaded when the
// underlying files change. It is safe for concurrent use.
type CertReloader struct {
	certPath string
	keyPath  string

	mu       sync.RWMutex
	cert     *tls.Certificate
	certMod  time.Time
	keyMod   time.Time
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewCertReloader creates a CertReloader that loads the certificate from
// certPath and keyPath. It starts a background goroutine that checks file
// modification times every 60 seconds and reloads if changed.
func NewCertReloader(certPath, keyPath string) (*CertReloader, error) {
	r := &CertReloader{
		certPath: certPath,
		keyPath:  keyPath,
		stopCh:   make(chan struct{}),
	}

	if err := r.loadCert(); err != nil {
		return nil, fmt.Errorf("initial cert load: %w", err)
	}

	go r.watchLoop()

	return r, nil
}

// GetCertificate implements the tls.Config.GetCertificate callback.
func (r *CertReloader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

// Stop terminates the background file watcher.
func (r *CertReloader) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
}

func (r *CertReloader) loadCert() error {
	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return err
	}

	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.cert = &cert
	r.certMod = certInfo.ModTime()
	r.keyMod = keyInfo.ModTime()
	r.mu.Unlock()

	return nil
}

func (r *CertReloader) checkAndReload() {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		log.Warn().Err(err).Str("path", r.certPath).Msg("cert file stat failed during reload check")
		return
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		log.Warn().Err(err).Str("path", r.keyPath).Msg("key file stat failed during reload check")
		return
	}

	r.mu.RLock()
	certChanged := !certInfo.ModTime().Equal(r.certMod)
	keyChanged := !keyInfo.ModTime().Equal(r.keyMod)
	r.mu.RUnlock()

	if !certChanged && !keyChanged {
		return
	}

	log.Info().
		Str("cert", r.certPath).
		Bool("cert_changed", certChanged).
		Bool("key_changed", keyChanged).
		Msg("TLS certificate file changed, reloading")

	if err := r.loadCert(); err != nil {
		log.Error().Err(err).Msg("TLS certificate reload failed, keeping previous certificate")
	} else {
		log.Info().Msg("TLS certificate reloaded successfully")
	}
}

func (r *CertReloader) watchLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.checkAndReload()
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestCertReloader" -v`

Expected: All 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/tls.go backend/internal/server/tls_test.go
git commit -m "feat(server): add TLS certificate reloader with automatic mtime watching"
```

---

### Task 4: Wire Static File Server and TLS into Main

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Update NewAppRouter to accept and mount static dir**

In `backend/internal/server/server.go`, modify the `NewAppRouter` function signature to accept a `staticDir string` parameter. Add it after the `opts AppOptions` parameter:

```go
func NewAppRouter(
	authService *app.AuthService,
	dashboardService *app.DashboardService,
	monitoringService *app.MonitoringService,
	settingsService *app.SettingsService,
	ocservService *app.OcservService,
	healthService *app.HealthService,
	logTailService *app.LogTailService,
	configService *app.ConfigManagementService,
	adminUserService *app.AdminUserService,
	auditLogService *app.AuditLogService,
	authorizer rbac.Authorizer,
	opts AppOptions,
	staticDir string,
) http.Handler {
```

At the end of the function, before `return securityHeaders(mux)`, add:

```go
	// Serve frontend static files with SPA fallback
	if staticDir != "" {
		mux.Handle("/", NewSPAFileServer(staticDir))
	}

	return securityHeaders(mux)
}
```

- [ ] **Step 2: Update all callers of NewAppRouter to pass staticDir**

In `backend/cmd/ocserv-panel/main.go`, update the `NewAppRouter` call to pass the static dir:

```go
staticDir := envOrDefault("OCSERV_PANEL_STATIC_DIR", cfg.Installation.StaticDir)

httpServer := &http.Server{
    Addr:              addr,
    Handler:           server.NewAppRouter(authService, dashboardService, monitoringService, settingsService, ocservService, healthService, logTailService, configMgmtService, adminUserService, auditLogService, rbac.NewDefaultAuthorizer(), server.AppOptions{CookieSecure: cookieSecureFromEnv(), LoginLimiter: loginLimiter}, staticDir),
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      60 * time.Second,
}
```

In test files that call `NewAppRouter`, pass `""` as the last argument (no static dir in tests).

- [ ] **Step 3: Add TLS support to main.go**

In `backend/cmd/ocserv-panel/main.go`, add TLS imports at the top:

```go
"crypto/tls"
```

Replace the server startup section (after `httpServer` creation) with:

```go
// Resolve TLS cert paths from env (overrides config)
tlsCert := envOrDefault("OCSERV_PANEL_TLS_CERT", cfg.Security.TLSCertPath)
tlsKey := envOrDefault("OCSERV_PANEL_TLS_KEY", cfg.Security.TLSKeyPath)

go func() {
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := httpServer.Shutdown(shutdownCtx); err != nil {
        log.Error().Err(err).Msg("backend shutdown failed")
    }
}()

if tlsCert != "" && tlsKey != "" {
    certReloader, certErr := server.NewCertReloader(tlsCert, tlsKey)
    if certErr != nil {
        log.Error().Err(certErr).Msg("failed to load TLS certificates")
        os.Exit(1)
    }
    defer certReloader.Stop()

    httpServer.TLSConfig = &tls.Config{
        GetCertificate: certReloader.GetCertificate,
        MinVersion:     tls.VersionTLS12,
    }

    log.Info().
        Str("addr", addr).
        Str("tls_cert", tlsCert).
        Str("static_dir", staticDir).
        Msg("starting ocserv panel backend (HTTPS)")
    if err := httpServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Error().Err(err).Msg("backend server failed")
        os.Exit(1)
    }
} else {
    log.Info().
        Str("addr", addr).
        Str("static_dir", staticDir).
        Msg("starting ocserv panel backend (HTTP)")
    if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Error().Err(err).Msg("backend server failed")
        os.Exit(1)
    }
}
```

- [ ] **Step 4: Add version variable for build-time injection**

At the top of `backend/cmd/ocserv-panel/main.go`, add:

```go
var version = "dev"
```

Add a `--version` flag check early in main:

```go
func main() {
    if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
        fmt.Println("ocserv-panel", version)
        os.Exit(0)
    }
    // ... rest of main
```

Add `"fmt"` to imports if not already present.

- [ ] **Step 5: Fix test compilation**

Update any test file that calls `NewAppRouter` (e.g., `server_test.go`) to pass `""` as the `staticDir` argument.

Run: `cd /opt/ocserv_panel/backend && go build ./...`

Expected: Compiles cleanly.

- [ ] **Step 6: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./...`

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/server/server.go backend/cmd/ocserv-panel/main.go
git commit -m "feat: wire static file server and TLS support into main server"
```

---

### Task 5: Switch Frontend to adapter-static

**Files:**
- Modify: `frontend/svelte.config.js`
- Modify: `frontend/package.json`
- Create: `frontend/src/routes/+layout.ts`

- [ ] **Step 1: Install adapter-static and remove adapter-auto**

Run: `cd /opt/ocserv_panel/frontend && pnpm remove @sveltejs/adapter-auto && pnpm add -D @sveltejs/adapter-static`

- [ ] **Step 2: Update svelte.config.js**

Replace `frontend/svelte.config.js` entirely:

```javascript
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html',
      precompress: true
    })
  }
};

export default config;
```

- [ ] **Step 3: Create layout config for SPA mode**

Create `frontend/src/routes/+layout.ts`:

```typescript
export const prerender = true;
export const ssr = false;
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd /opt/ocserv_panel/frontend && pnpm build`

Expected: Build completes. Output in `frontend/build/` with `index.html`, `_app/` directory, and `.gz`/`.br` pre-compressed files.

- [ ] **Step 5: Verify build output structure**

Run: `ls /opt/ocserv_panel/frontend/build/index.html && ls /opt/ocserv_panel/frontend/build/_app/ | head -5 && ls /opt/ocserv_panel/frontend/build/*.gz 2>/dev/null | head -3`

Expected: `index.html` exists, `_app/` has files, `.gz` files present.

- [ ] **Step 6: Commit**

```bash
git add frontend/svelte.config.js frontend/package.json frontend/pnpm-lock.yaml frontend/src/routes/+layout.ts
git commit -m "feat(frontend): switch to adapter-static for SPA deployment"
```

---

### Task 6: Makefile for Build Pipeline

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Create the Makefile**

Create `Makefile` at the project root (`/opt/ocserv_panel/Makefile`):

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := ocserv-panel
DIST    := dist

.PHONY: all clean build-backend build-frontend release install-deps check

all: release

# ─── Backend ────────────────────────────────────────────────────────────────

build-backend:
	@echo "→ Building Go backend ($(VERSION))..."
	@mkdir -p $(DIST)/bin
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o ../$(DIST)/bin/$(BINARY) ./cmd/ocserv-panel
	@echo "  ✓ $(DIST)/bin/$(BINARY)"

# ─── Frontend ───────────────────────────────────────────────────────────────

build-frontend:
	@echo "→ Building SvelteKit frontend..."
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	@mkdir -p $(DIST)/static
	cp -r frontend/build/* $(DIST)/static/
	@echo "  ✓ $(DIST)/static/"

# ─── Release Tarball ────────────────────────────────────────────────────────

release: clean build-backend build-frontend
	@echo "→ Packaging release tarball..."
	@mkdir -p $(DIST)/migrations $(DIST)/sudoers.d
	cp migrations/*.sql $(DIST)/migrations/
	cp deploy/sudoers.d/ocserv-panel $(DIST)/sudoers.d/
	cp deploy/ocserv-panel.service $(DIST)/
	cp deploy/install.sh $(DIST)/
	chmod +x $(DIST)/install.sh
	cp deploy/config.toml.example $(DIST)/
	echo "$(VERSION)" > $(DIST)/VERSION
	tar -czf $(BINARY)-$(VERSION)-linux-amd64.tar.gz -C $(DIST) .
	@echo "  ✓ $(BINARY)-$(VERSION)-linux-amd64.tar.gz"
	@echo ""
	@echo "Release artifact ready. Transfer to target and run:"
	@echo "  tar xzf $(BINARY)-$(VERSION)-linux-amd64.tar.gz -C /tmp/ocserv-panel-install"
	@echo "  cd /tmp/ocserv-panel-install && sudo ./install.sh"

# ─── Utilities ──────────────────────────────────────────────────────────────

clean:
	rm -rf $(DIST) $(BINARY)-*-linux-amd64.tar.gz frontend/build

check:
	@echo "→ Running backend tests..."
	cd backend && go test ./...
	@echo "→ Running frontend checks..."
	cd frontend && pnpm check

install-deps:
	cd frontend && pnpm install
```

- [ ] **Step 2: Verify Makefile syntax**

Run: `cd /opt/ocserv_panel && make --dry-run release 2>&1 | head -20`

Expected: Shows the commands that would be run without errors.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile with backend, frontend, and release targets"
```

---

### Task 7: Update Systemd Unit and Sudoers

**Files:**
- Modify: `deploy/ocserv-panel.service`
- Modify: `deploy/sudoers.d/ocserv-panel`

- [ ] **Step 1: Update the systemd unit file**

Replace `deploy/ocserv-panel.service` with the full production-ready version:

```ini
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
After=network-online.target ocserv.service
Wants=network-online.target ocserv.service

[Service]
Type=simple
User=ocserv-panel
Group=ocserv-panel
WorkingDirectory=/var/lib/ocserv-panel

ExecStart=/usr/local/bin/ocserv-panel

Environment="OCSERV_PANEL_ADDR=0.0.0.0:8443"
Environment="OCSERV_PANEL_DB_DSN=file:/var/lib/ocserv-panel/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=/etc/ocserv-panel/config.toml"
Environment="OCSERV_PANEL_TSDB_PATH=/var/lib/ocserv-panel/metrics"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=/usr/share/ocserv-panel/migrations"
Environment="OCSERV_PANEL_STATIC_DIR=/usr/share/ocserv-panel/static"
Environment="OCSERV_PANEL_TLS_CERT=/etc/letsencrypt/live/light-mirror.ru/fullchain.pem"
Environment="OCSERV_PANEL_TLS_KEY=/etc/letsencrypt/live/light-mirror.ru/privkey.pem"
Environment="OCSERV_PANEL_COOKIE_SECURE=true"

Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ocserv-panel /var/log/ocserv-panel
ReadOnlyPaths=/etc/ocserv-panel /etc/ocserv /etc/letsencrypt /usr/share/ocserv-panel

NoNewPrivileges=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Update the sudoers file**

Replace `deploy/sudoers.d/ocserv-panel` with the full allowlist:

```
# Sudoers rules for the ocserv-panel service account
# Install: sudo cp deploy/sudoers.d/ocserv-panel /etc/sudoers.d/ocserv-panel
#          sudo chmod 440 /etc/sudoers.d/ocserv-panel
#          sudo visudo -c

# Service management
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl reload ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl status ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-enabled ocserv

# Session and status queries
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl disconnect id *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show users
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show user *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show status
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl reload

# Password file management
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -d *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -l *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -u *
```

- [ ] **Step 3: Commit**

```bash
git add deploy/ocserv-panel.service deploy/sudoers.d/ocserv-panel
git commit -m "deploy: update systemd unit for TLS/static serving and expand sudoers allowlist"
```

---

### Task 8: Create config.toml.example

**Files:**
- Create: `deploy/config.toml.example`

- [ ] **Step 1: Write the reference configuration**

Create `deploy/config.toml.example`:

```toml
# ocserv-panel configuration
# Copy to /etc/ocserv-panel/config.toml and adjust paths for your system.
# The installer generates this file automatically with detected values.

[general]
app_name = "ocserv Panel"
timezone = "UTC"

[installation]
config_path = "/etc/ocserv-panel/config.toml"
data_dir = "/var/lib/ocserv-panel"
log_dir = "/var/log/ocserv-panel"
static_dir = "/usr/share/ocserv-panel/static"

# ocserv paths — adjust if your system uses different locations
ocserv_config_path = "/etc/ocserv/ocserv.conf"
ocserv_socket_path = "/var/run/ocserv-socket"
ocserv_bin_path = "/usr/sbin/ocserv"
occtl_bin_path = "/usr/bin/occtl"
ocpasswd_bin_path = "/usr/bin/ocpasswd"
ocpasswd_file_path = "/etc/ocserv/ocpasswd"

# Certificate directory (used for display/expiry checks)
cert_dir = "/etc/letsencrypt/live/example.com"

# Log file paths
ocserv_log_path = "/var/log/ocserv/ocserv.log"
panel_log_path = "/var/log/ocserv-panel/panel.log"
panel_systemd_unit = "ocserv-panel"

[service]
name = "ocserv-panel"
user = "ocserv-panel"
group = "ocserv-panel"
start_on_boot = true
systemd_unit_name = "ocserv"

[web]
host = "0.0.0.0"
port = 8443
base_url = "https://your-domain.com:8443"

[security]
# TLS certificate and key paths (Let's Encrypt recommended)
# Leave empty to run in HTTP-only mode (not recommended for production)
tls_cert_path = "/etc/letsencrypt/live/example.com/fullchain.pem"
tls_key_path = "/etc/letsencrypt/live/example.com/privkey.pem"
csrf_enabled = true
security_headers = true

[database]
dsn = "file:/var/lib/ocserv-panel/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
max_open_conns = 5
max_idle_conns = 2
conn_max_lifetime = 300

[monitoring]
health_endpoint = true
dashboard_refresh_interval = 15
log_file_read_enabled = true
```

- [ ] **Step 2: Commit**

```bash
git add deploy/config.toml.example
git commit -m "deploy: add config.toml.example reference configuration"
```

---

### Task 9: Interactive Install Script

**Files:**
- Create: `deploy/install.sh`

- [ ] **Step 1: Write the install script**

Create `deploy/install.sh` — a complete interactive installer that handles fresh install, upgrade, and uninstall:

```bash
#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# ocserv-panel installer
# ═══════════════════════════════════════════════════════════════════════════════

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly VERSION="$(cat "${SCRIPT_DIR}/VERSION" 2>/dev/null || echo "unknown")"

# Installation paths
readonly BIN_DIR="/usr/local/bin"
readonly SHARE_DIR="/usr/share/ocserv-panel"
readonly CONFIG_DIR="/etc/ocserv-panel"
readonly DATA_DIR="/var/lib/ocserv-panel"
readonly LOG_DIR="/var/log/ocserv-panel"
readonly SYSTEMD_DIR="/etc/systemd/system"
readonly SUDOERS_DIR="/etc/sudoers.d"
readonly SERVICE_USER="ocserv-panel"
readonly SERVICE_NAME="ocserv-panel"

# Colors
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly BOLD='\033[1m'
readonly NC='\033[0m'

# ─── Helpers ────────────────────────────────────────────────────────────────

info()    { echo -e "  ${GREEN}[✓]${NC} $*"; }
warn()    { echo -e "  ${YELLOW}[!]${NC} $*"; }
error()   { echo -e "  ${RED}[✗]${NC} $*"; }
step()    { echo -e "\n${BLUE}→${NC} ${BOLD}$*${NC}"; }
header()  { echo -e "\n${BOLD}╔══════════════════════════════════════════════════════════╗${NC}"; echo -e "${BOLD}║  ocserv-panel installer v${VERSION}$(printf '%*s' $((34 - ${#VERSION})) '')║${NC}"; echo -e "${BOLD}╚══════════════════════════════════════════════════════════╝${NC}"; }

confirm() {
    local prompt="${1:-Continue?}"
    echo -en "\n  ${prompt} [Y/n] "
    read -r answer
    [[ -z "$answer" || "$answer" =~ ^[Yy] ]]
}

die() {
    error "$1"
    echo ""
    if [[ -n "${2:-}" ]]; then
        echo -e "  ${BOLD}Remediation:${NC} $2"
    fi
    exit 1
}

# ─── Detection ──────────────────────────────────────────────────────────────

detect_ocserv() {
    OCSERV_BIN="$(command -v ocserv 2>/dev/null || echo "")"
    OCCTL_BIN="$(command -v occtl 2>/dev/null || echo "")"
    OCPASSWD_BIN="$(command -v ocpasswd 2>/dev/null || echo "")"

    # Find config path from running process or default
    if systemctl is-active --quiet ocserv 2>/dev/null; then
        OCSERV_CONFIG="$(systemctl show ocserv -p ExecStart --value 2>/dev/null | grep -oP '(?<=--config )\S+' || echo "/etc/ocserv/ocserv.conf")"
        OCSERV_RUNNING=true
    else
        OCSERV_CONFIG="/etc/ocserv/ocserv.conf"
        OCSERV_RUNNING=false
    fi
    [[ -z "$OCSERV_CONFIG" ]] && OCSERV_CONFIG="/etc/ocserv/ocserv.conf"

    # Detect socket path from config
    if [[ -f "$OCSERV_CONFIG" ]]; then
        OCSERV_SOCKET="$(grep -E '^\s*socket-file\s*=' "$OCSERV_CONFIG" | sed 's/.*=\s*//' | tr -d ' "' || echo "/var/run/ocserv-socket")"
        OCSERV_PASSWD="$(grep -E '^\s*auth\s*=.*plain\[' "$OCSERV_CONFIG" | grep -oP 'passwd=\K[^\],]+' || echo "/etc/ocserv/ocpasswd")"
    else
        OCSERV_SOCKET="/var/run/ocserv-socket"
        OCSERV_PASSWD="/etc/ocserv/ocpasswd"
    fi

    # Detect Let's Encrypt certs from ocserv config
    if [[ -f "$OCSERV_CONFIG" ]]; then
        CERT_PATH="$(grep -E '^\s*server-cert\s*=' "$OCSERV_CONFIG" | head -1 | sed 's/.*=\s*//' | tr -d ' "' || echo "")"
        KEY_PATH="$(grep -E '^\s*server-key\s*=' "$OCSERV_CONFIG" | head -1 | sed 's/.*=\s*//' | tr -d ' "' || echo "")"
    fi
    CERT_PATH="${CERT_PATH:-}"
    KEY_PATH="${KEY_PATH:-}"
}

detect_existing() {
    INSTALLED_VERSION=""
    if [[ -f "${SHARE_DIR}/VERSION" ]]; then
        INSTALLED_VERSION="$(cat "${SHARE_DIR}/VERSION")"
    fi
}

# ─── Preflight ──────────────────────────────────────────────────────────────

preflight() {
    step "Preflight checks"

    # Root check
    [[ $EUID -eq 0 ]] || die "Must run as root" "Use: sudo ./install.sh"
    info "Running as root"

    # systemd check
    command -v systemctl >/dev/null 2>&1 || die "systemd not available" "This installer requires systemd"
    info "systemd available"

    # Required artifacts
    [[ -f "${SCRIPT_DIR}/bin/ocserv-panel" ]] || die "Binary not found: bin/ocserv-panel" "Ensure you extracted the full release tarball"
    [[ -d "${SCRIPT_DIR}/static" ]] || die "Static files not found: static/" "Ensure you extracted the full release tarball"
    [[ -d "${SCRIPT_DIR}/migrations" ]] || die "Migrations not found: migrations/" "Ensure you extracted the full release tarball"
    info "Required files present (bin/ocserv-panel, static/, migrations/)"

    # Detect ocserv
    detect_ocserv

    if [[ -n "$OCSERV_BIN" ]]; then
        info "ocserv binary: ${OCSERV_BIN}"
    else
        warn "ocserv binary not found in PATH"
    fi

    if [[ -n "$OCCTL_BIN" ]]; then
        info "occtl: ${OCCTL_BIN}"
    else
        warn "occtl not found — session management will not work"
    fi

    if [[ -n "$OCPASSWD_BIN" ]]; then
        info "ocpasswd: ${OCPASSWD_BIN}"
    else
        warn "ocpasswd not found — VPN user management will not work"
    fi

    if [[ -f "$OCSERV_CONFIG" ]]; then
        info "ocserv config: ${OCSERV_CONFIG}"
    else
        warn "ocserv config not found: ${OCSERV_CONFIG}"
    fi

    if [[ -f "$OCSERV_PASSWD" ]]; then
        info "ocserv password file: ${OCSERV_PASSWD}"
    else
        warn "ocserv password file not found: ${OCSERV_PASSWD}"
    fi

    if [[ "$OCSERV_RUNNING" == "true" ]]; then
        info "ocserv service: active (running)"
    else
        warn "ocserv service: not running"
    fi

    # TLS certs
    if [[ -n "$CERT_PATH" && -f "$CERT_PATH" ]]; then
        info "TLS certificate: ${CERT_PATH}"
    elif [[ -n "$CERT_PATH" ]]; then
        warn "TLS certificate configured but missing: ${CERT_PATH}"
    else
        warn "No TLS certificate detected — panel will start in HTTP mode"
    fi

    if [[ -n "$KEY_PATH" && -f "$KEY_PATH" ]]; then
        info "TLS key: ${KEY_PATH}"
    elif [[ -n "$KEY_PATH" ]]; then
        warn "TLS key configured but missing: ${KEY_PATH}"
    fi

    # Existing installation
    detect_existing
    if [[ -n "$INSTALLED_VERSION" ]]; then
        if [[ "$INSTALLED_VERSION" == "$VERSION" ]]; then
            warn "Same version already installed (v${INSTALLED_VERSION}) — will reinstall"
        else
            warn "Upgrade detected: v${INSTALLED_VERSION} → v${VERSION}"
        fi
        UPGRADE=true
    else
        info "Fresh installation"
        UPGRADE=false
    fi
}

# ─── Installation Steps ─────────────────────────────────────────────────────

show_plan() {
    echo ""
    echo -e "  ${BOLD}Installation plan:${NC}"
    local n=1
    if [[ "$UPGRADE" == "true" ]]; then
        echo "    ${n}. Stop existing service"; ((n++))
        echo "    ${n}. Backup database and config"; ((n++))
    fi
    echo "    ${n}. Create/verify service account: ${SERVICE_USER}"; ((n++))
    echo "    ${n}. Install binary: ${BIN_DIR}/ocserv-panel"; ((n++))
    echo "    ${n}. Install static files: ${SHARE_DIR}/static/"; ((n++))
    echo "    ${n}. Install migrations: ${SHARE_DIR}/migrations/"; ((n++))
    echo "    ${n}. Generate config: ${CONFIG_DIR}/config.toml"; ((n++))
    echo "    ${n}. Install sudoers rules: ${SUDOERS_DIR}/ocserv-panel"; ((n++))
    echo "    ${n}. Verify cert access for ${SERVICE_USER}"; ((n++))
    echo "    ${n}. Install systemd unit"; ((n++))
    echo "    ${n}. Enable and start service"; ((n++))
    echo "    ${n}. Verify health"; ((n++))
}

stop_existing() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        step "Stopping existing service"
        systemctl stop "$SERVICE_NAME"
        info "Service stopped"
    fi
}

backup_data() {
    step "Backing up data"
    local backup_name="backup-$(date +%Y%m%d-%H%M%S).tar.gz"
    local backup_path="${DATA_DIR}/${backup_name}"
    local files_to_backup=()

    [[ -f "${DATA_DIR}/ocserv_panel.db" ]] && files_to_backup+=("${DATA_DIR}/ocserv_panel.db")
    [[ -f "${CONFIG_DIR}/config.toml" ]] && files_to_backup+=("${CONFIG_DIR}/config.toml")

    if [[ ${#files_to_backup[@]} -gt 0 ]]; then
        tar -czf "$backup_path" "${files_to_backup[@]}" 2>/dev/null || true
        info "Backup saved: ${backup_path}"
    else
        info "No existing data to backup"
    fi
}

create_user() {
    step "Service account"
    if id "$SERVICE_USER" &>/dev/null; then
        info "User '${SERVICE_USER}' already exists"
    else
        useradd --system --shell /usr/sbin/nologin --no-create-home \
            --comment "ocserv-panel web UI" "$SERVICE_USER"
        info "Created user: ${SERVICE_USER}"
    fi
}

install_binary() {
    step "Installing binary"
    install -m 755 "${SCRIPT_DIR}/bin/ocserv-panel" "${BIN_DIR}/ocserv-panel"
    info "${BIN_DIR}/ocserv-panel ($(${BIN_DIR}/ocserv-panel --version 2>&1 || echo v${VERSION}))"
}

install_static() {
    step "Installing static files"
    mkdir -p "${SHARE_DIR}/static"
    rm -rf "${SHARE_DIR}/static/"*
    cp -r "${SCRIPT_DIR}/static/"* "${SHARE_DIR}/static/"
    local file_count
    file_count=$(find "${SHARE_DIR}/static" -type f | wc -l)
    info "${SHARE_DIR}/static/ (${file_count} files)"
}

install_migrations() {
    step "Installing migrations"
    mkdir -p "${SHARE_DIR}/migrations"
    cp "${SCRIPT_DIR}/migrations/"*.sql "${SHARE_DIR}/migrations/"
    echo "$VERSION" > "${SHARE_DIR}/VERSION"
    local mig_count
    mig_count=$(ls "${SHARE_DIR}/migrations/"*.sql 2>/dev/null | wc -l)
    info "${SHARE_DIR}/migrations/ (${mig_count} files)"
}

generate_config() {
    step "Configuration"
    mkdir -p "$CONFIG_DIR"

    if [[ -f "${CONFIG_DIR}/config.toml" ]]; then
        info "Existing config preserved: ${CONFIG_DIR}/config.toml"
        return
    fi

    cat > "${CONFIG_DIR}/config.toml" <<TOML
[general]
app_name = "ocserv Panel"
timezone = "UTC"

[installation]
config_path = "${CONFIG_DIR}/config.toml"
data_dir = "${DATA_DIR}"
log_dir = "${LOG_DIR}"
static_dir = "${SHARE_DIR}/static"
ocserv_config_path = "${OCSERV_CONFIG}"
ocserv_socket_path = "${OCSERV_SOCKET}"
ocserv_bin_path = "${OCSERV_BIN:-/usr/sbin/ocserv}"
occtl_bin_path = "${OCCTL_BIN:-/usr/bin/occtl}"
ocpasswd_bin_path = "${OCPASSWD_BIN:-/usr/bin/ocpasswd}"
ocpasswd_file_path = "${OCSERV_PASSWD}"
cert_dir = "$(dirname "${CERT_PATH:-/etc/letsencrypt/live/example.com/fullchain.pem}")"
panel_log_path = "${LOG_DIR}/panel.log"
panel_systemd_unit = "${SERVICE_NAME}"

[service]
name = "${SERVICE_NAME}"
user = "${SERVICE_USER}"
group = "${SERVICE_USER}"
start_on_boot = true
systemd_unit_name = "ocserv"

[web]
host = "0.0.0.0"
port = 8443

[security]
tls_cert_path = "${CERT_PATH:-}"
tls_key_path = "${KEY_PATH:-}"
csrf_enabled = true
security_headers = true

[database]
dsn = "file:${DATA_DIR}/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
max_open_conns = 5
max_idle_conns = 2
conn_max_lifetime = 300

[monitoring]
health_endpoint = true
dashboard_refresh_interval = 15
log_file_read_enabled = true
TOML

    chown root:"$SERVICE_USER" "${CONFIG_DIR}/config.toml"
    chmod 640 "${CONFIG_DIR}/config.toml"
    info "Generated: ${CONFIG_DIR}/config.toml"
}

install_sudoers() {
    step "Sudoers rules"
    local systemctl_path
    systemctl_path="$(command -v systemctl)"
    local occtl_path="${OCCTL_BIN:-/usr/bin/occtl}"
    local ocpasswd_path="${OCPASSWD_BIN:-/usr/bin/ocpasswd}"

    cat > "${SUDOERS_DIR}/ocserv-panel" <<SUDOERS
# ocserv-panel service account — auto-generated by installer
# Service management
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} start ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} stop ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} restart ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} reload ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} status ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} is-active ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_path} is-enabled ocserv

# Session and status queries
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_path} disconnect id *
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_path} show users
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_path} show user *
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_path} show status
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_path} reload

# Password file management
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_path} -c ${OCSERV_PASSWD} *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_path} -c ${OCSERV_PASSWD} -d *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_path} -c ${OCSERV_PASSWD} -l *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_path} -c ${OCSERV_PASSWD} -u *
SUDOERS

    chmod 440 "${SUDOERS_DIR}/ocserv-panel"

    if visudo -c -f "${SUDOERS_DIR}/ocserv-panel" >/dev/null 2>&1; then
        info "Sudoers rules installed and validated"
    else
        rm -f "${SUDOERS_DIR}/ocserv-panel"
        die "Sudoers syntax validation failed" "Check paths and try again"
    fi
}

verify_cert_access() {
    step "Certificate access"
    if [[ -z "$CERT_PATH" || ! -f "$CERT_PATH" ]]; then
        warn "No TLS certificate configured — panel will use HTTP"
        warn "Set tls_cert_path and tls_key_path in ${CONFIG_DIR}/config.toml for HTTPS"
        return
    fi

    # Check if the service user can read the cert
    if sudo -u "$SERVICE_USER" test -r "$CERT_PATH" 2>/dev/null; then
        info "User '${SERVICE_USER}' can read: ${CERT_PATH}"
    else
        warn "User '${SERVICE_USER}' cannot read: ${CERT_PATH}"
        echo ""
        echo -e "  ${BOLD}Grant access with one of:${NC}"
        echo "    setfacl -m u:${SERVICE_USER}:rX $(dirname $(dirname "$CERT_PATH"))"
        echo "    setfacl -m u:${SERVICE_USER}:rX $(dirname "$CERT_PATH")"
        echo "    setfacl -m u:${SERVICE_USER}:r ${CERT_PATH}"
        echo "    setfacl -m u:${SERVICE_USER}:r ${KEY_PATH}"
        echo ""
        if ! confirm "Continue without cert access? (panel will fail to start with TLS)"; then
            die "Aborted — fix certificate permissions first"
        fi
    fi

    if [[ -n "$KEY_PATH" && -f "$KEY_PATH" ]]; then
        if sudo -u "$SERVICE_USER" test -r "$KEY_PATH" 2>/dev/null; then
            info "User '${SERVICE_USER}' can read: ${KEY_PATH}"
        else
            warn "User '${SERVICE_USER}' cannot read: ${KEY_PATH}"
        fi
    fi
}

install_systemd() {
    step "Systemd unit"

    # Generate unit with detected paths
    cat > "${SYSTEMD_DIR}/${SERVICE_NAME}.service" <<UNIT
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
After=network-online.target ocserv.service
Wants=network-online.target ocserv.service

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${DATA_DIR}

ExecStart=${BIN_DIR}/ocserv-panel

Environment="OCSERV_PANEL_ADDR=0.0.0.0:8443"
Environment="OCSERV_PANEL_DB_DSN=file:${DATA_DIR}/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=${CONFIG_DIR}/config.toml"
Environment="OCSERV_PANEL_TSDB_PATH=${DATA_DIR}/metrics"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=${SHARE_DIR}/migrations"
Environment="OCSERV_PANEL_STATIC_DIR=${SHARE_DIR}/static"
Environment="OCSERV_PANEL_TLS_CERT=${CERT_PATH:-}"
Environment="OCSERV_PANEL_TLS_KEY=${KEY_PATH:-}"
Environment="OCSERV_PANEL_COOKIE_SECURE=true"

Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR}
ReadOnlyPaths=${CONFIG_DIR} /etc/ocserv /etc/letsencrypt ${SHARE_DIR}

NoNewPrivileges=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
UNIT

    # Create data and log dirs
    mkdir -p "$DATA_DIR" "$LOG_DIR"
    chown "${SERVICE_USER}:${SERVICE_USER}" "$DATA_DIR" "$LOG_DIR"
    chmod 750 "$DATA_DIR" "$LOG_DIR"

    # Set config dir permissions
    chown "root:${SERVICE_USER}" "$CONFIG_DIR"
    chmod 750 "$CONFIG_DIR"

    systemctl daemon-reload
    info "Unit installed: ${SYSTEMD_DIR}/${SERVICE_NAME}.service"
}

start_service() {
    step "Starting service"
    systemctl enable "$SERVICE_NAME" --now
    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        info "Service active: ${SERVICE_NAME}"
    else
        error "Service failed to start"
        echo ""
        echo -e "  ${BOLD}Debug:${NC}"
        echo "    journalctl -u ${SERVICE_NAME} --no-pager -n 20"
        echo "    systemctl status ${SERVICE_NAME}"
        exit 1
    fi
}

verify_health() {
    step "Post-install verification"

    # Check if listening
    if command -v ss >/dev/null 2>&1; then
        if ss -tlnp | grep -q ":8443"; then
            info "Listening on 0.0.0.0:8443"
        else
            warn "Not detected on port 8443 (might need a moment)"
        fi
    fi

    # Health check
    local scheme="https"
    if [[ -z "$CERT_PATH" || ! -f "$CERT_PATH" ]]; then
        scheme="http"
    fi

    if command -v curl >/dev/null 2>&1; then
        local health_url="${scheme}://localhost:8443/api/health"
        if curl -sk --max-time 5 "$health_url" | grep -q '"ok"'; then
            info "Health check: ${health_url} → 200 OK"
        else
            warn "Health check failed (service may still be initializing)"
        fi
    else
        warn "curl not found — skipping health check"
    fi

    echo ""
    echo -e "  ${BOLD}════════════════════════════════════════════════════════${NC}"
    echo -e "  ${GREEN}Installation complete!${NC}"
    echo ""
    echo -e "  Access the panel at: ${BOLD}${scheme}://$(hostname -f 2>/dev/null || hostname):8443${NC}"
    echo -e "  First login: create admin account at the setup page"
    echo ""
    echo -e "  ${BOLD}Useful commands:${NC}"
    echo "    systemctl status ${SERVICE_NAME}"
    echo "    journalctl -u ${SERVICE_NAME} -f"
    echo "    ${BIN_DIR}/ocserv-panel --version"
    echo -e "  ${BOLD}════════════════════════════════════════════════════════${NC}"
}

# ─── Uninstall ──────────────────────────────────────────────────────────────

uninstall() {
    header
    step "Uninstalling ocserv-panel"

    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl stop "$SERVICE_NAME"
        info "Service stopped"
    fi
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    info "Service disabled"

    rm -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service"
    info "Removed systemd unit"

    rm -f "${SUDOERS_DIR}/ocserv-panel"
    info "Removed sudoers rules"

    rm -f "${BIN_DIR}/ocserv-panel"
    info "Removed binary"

    rm -rf "${SHARE_DIR}"
    info "Removed static files and migrations"

    systemctl daemon-reload

    echo ""
    if confirm "Remove data directory (${DATA_DIR})? This deletes the database and metrics."; then
        rm -rf "$DATA_DIR"
        info "Removed: ${DATA_DIR}"
    else
        info "Kept: ${DATA_DIR}"
    fi

    if confirm "Remove config directory (${CONFIG_DIR})?"; then
        rm -rf "$CONFIG_DIR"
        info "Removed: ${CONFIG_DIR}"
    else
        info "Kept: ${CONFIG_DIR}"
    fi

    if confirm "Remove log directory (${LOG_DIR})?"; then
        rm -rf "$LOG_DIR"
        info "Removed: ${LOG_DIR}"
    else
        info "Kept: ${LOG_DIR}"
    fi

    if id "$SERVICE_USER" &>/dev/null; then
        if confirm "Remove system user '${SERVICE_USER}'?"; then
            userdel "$SERVICE_USER" 2>/dev/null || true
            info "Removed user: ${SERVICE_USER}"
        else
            info "Kept user: ${SERVICE_USER}"
        fi
    fi

    echo ""
    echo -e "  ${GREEN}Uninstall complete.${NC}"
}

# ─── Main ───────────────────────────────────────────────────────────────────

main() {
    case "${1:-}" in
        --uninstall)
            uninstall
            exit 0
            ;;
        --help|-h)
            echo "Usage: $0 [--uninstall] [--help]"
            echo ""
            echo "  (no args)    Install or upgrade ocserv-panel"
            echo "  --uninstall  Remove ocserv-panel from the system"
            exit 0
            ;;
    esac

    header
    preflight

    show_plan
    confirm "Proceed with installation?" || exit 0

    if [[ "$UPGRADE" == "true" ]]; then
        stop_existing
        backup_data
    fi

    create_user
    install_binary
    install_static
    install_migrations
    generate_config
    install_sudoers
    verify_cert_access
    install_systemd
    start_service
    verify_health
}

main "$@"
```

- [ ] **Step 2: Make the script executable**

Run: `chmod +x /opt/ocserv_panel/deploy/install.sh`

- [ ] **Step 3: Validate shell syntax**

Run: `bash -n /opt/ocserv_panel/deploy/install.sh`

Expected: No output (no syntax errors).

- [ ] **Step 4: Commit**

```bash
git add deploy/install.sh
git commit -m "deploy: add interactive install/upgrade/uninstall script"
```

---

### Task 10: Build and Verify Full Release

**Files:**
- None new — this task validates the entire pipeline.

- [ ] **Step 1: Run backend tests**

Run: `cd /opt/ocserv_panel/backend && go test ./...`

Expected: All packages pass.

- [ ] **Step 2: Run frontend build**

Run: `cd /opt/ocserv_panel/frontend && pnpm install && pnpm build`

Expected: Build succeeds, `frontend/build/` contains `index.html` and `_app/`.

- [ ] **Step 3: Build backend binary**

Run: `cd /opt/ocserv_panel && make build-backend`

Expected: `dist/bin/ocserv-panel` exists and is executable.

- [ ] **Step 4: Verify binary version flag**

Run: `./dist/bin/ocserv-panel --version`

Expected: Prints version string (e.g., `ocserv-panel dev` or a git tag).

- [ ] **Step 5: Build full release tarball**

Run: `cd /opt/ocserv_panel && make release`

Expected: Creates `ocserv-panel-<version>-linux-amd64.tar.gz`.

- [ ] **Step 6: Verify tarball contents**

Run: `tar -tzf /opt/ocserv_panel/ocserv-panel-*-linux-amd64.tar.gz | head -20`

Expected: Contains `bin/ocserv-panel`, `static/index.html`, `migrations/`, `install.sh`, `VERSION`, `config.toml.example`, `sudoers.d/ocserv-panel`, `ocserv-panel.service`.

- [ ] **Step 7: Commit any final adjustments**

If there are fixes needed from verification, commit them:

```bash
git add -A
git commit -m "fix: adjustments from release build verification"
```

---

## Summary of Commits

| Task | Commit Message |
|------|---------------|
| 1 | `feat(config): add StaticDir field and TLS env var merging` |
| 2 | `feat(server): add SPA file server with pre-compression and cache headers` |
| 3 | `feat(server): add TLS certificate reloader with automatic mtime watching` |
| 4 | `feat: wire static file server and TLS support into main server` |
| 5 | `feat(frontend): switch to adapter-static for SPA deployment` |
| 6 | `build: add Makefile with backend, frontend, and release targets` |
| 7 | `deploy: update systemd unit for TLS/static serving and expand sudoers allowlist` |
| 8 | `deploy: add config.toml.example reference configuration` |
| 9 | `deploy: add interactive install/upgrade/uninstall script` |
| 10 | (verification only, fix commit if needed) |
