package server

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const certReloadInterval = 60 * time.Second

// CertReloader holds a TLS certificate and reloads it automatically when the
// underlying files change on disk. It is designed for use with certbot renewals
// where the files are replaced atomically without restarting the process.
type CertReloader struct {
	certPath string
	keyPath  string

	mu      sync.RWMutex
	cert    *tls.Certificate
	certMod time.Time
	keyMod  time.Time

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewCertReloader loads the initial certificate from certPath and keyPath and
// starts a background goroutine that checks for file changes every 60 seconds.
// It returns an error if either file is missing or contains an invalid cert/key
// pair.
func NewCertReloader(certPath, keyPath string) (*CertReloader, error) {
	r := &CertReloader{
		certPath: certPath,
		keyPath:  keyPath,
		stopCh:   make(chan struct{}),
	}

	if err := r.load(); err != nil {
		return nil, fmt.Errorf("loading initial TLS certificate: %w", err)
	}

	go r.watch()

	return r, nil
}

// GetCertificate satisfies tls.Config.GetCertificate. It is safe to call
// concurrently from multiple goroutines.
func (r *CertReloader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

// Stop terminates the background watcher. It is safe to call multiple times.
func (r *CertReloader) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
}

// checkAndReload reads the current mtime of both cert and key files and reloads
// the certificate pair if either file has been modified since the last load. If
// the reload fails the previous valid certificate is kept.
func (r *CertReloader) checkAndReload() {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		log.Warn().Err(err).Str("path", r.certPath).Msg("tls reloader: cannot stat cert file")
		return
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		log.Warn().Err(err).Str("path", r.keyPath).Msg("tls reloader: cannot stat key file")
		return
	}

	r.mu.RLock()
	unchanged := !certInfo.ModTime().After(r.certMod) && !keyInfo.ModTime().After(r.keyMod)
	r.mu.RUnlock()

	if unchanged {
		return
	}

	if err := r.load(); err != nil {
		log.Error().Err(err).Msg("tls reloader: failed to reload certificate, keeping previous")
		return
	}

	log.Info().
		Str("cert", r.certPath).
		Str("key", r.keyPath).
		Msg("tls reloader: certificate reloaded")
}

// load reads the cert and key files, parses the certificate pair, and stores
// it under the write lock.
func (r *CertReloader) load() error {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		return fmt.Errorf("stat cert file %q: %w", r.certPath, err)
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		return fmt.Errorf("stat key file %q: %w", r.keyPath, err)
	}

	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return fmt.Errorf("loading key pair: %w", err)
	}

	r.mu.Lock()
	r.cert = &cert
	r.certMod = certInfo.ModTime()
	r.keyMod = keyInfo.ModTime()
	r.mu.Unlock()

	return nil
}

// watch is the background goroutine that triggers checkAndReload on a fixed
// interval until Stop is called.
func (r *CertReloader) watch() {
	ticker := time.NewTicker(certReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkAndReload()
		case <-r.stopCh:
			return
		}
	}
}
