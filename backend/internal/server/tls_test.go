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
	defer reloader.Stop()

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
	defer reloader.Stop()

	hello := &tls.ClientHelloInfo{ServerName: "test"}
	cert1, _ := reloader.GetCertificate(hello)

	// Regenerate the cert (different serial)
	time.Sleep(10 * time.Millisecond)
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
