package ocserv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestartDetector_PortChange(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("tcp-port = 443\nudp-port = 443\n", "tcp-port = 8443\nudp-port = 443\n")
	assert.True(t, result.RequiresRestart)
	require.Len(t, result.Reasons, 1)
	assert.Contains(t, result.Reasons[0], "tcp-port")
}

func TestRestartDetector_DNSChangeNoRestart(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("dns = 8.8.8.8\n", "dns = 1.1.1.1\n")
	assert.False(t, result.RequiresRestart)
	assert.Empty(t, result.Reasons)
}

func TestRestartDetector_CertPathChange(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("server-cert = /etc/ocserv/cert.pem\n", "server-cert = /etc/ocserv/new-cert.pem\n")
	assert.True(t, result.RequiresRestart)
	require.Len(t, result.Reasons, 1)
	assert.Contains(t, result.Reasons[0], "server-cert")
}

func TestRestartDetector_NoChanges(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("tcp-port = 443\ndevice = vpns\n", "tcp-port = 443\ndevice = vpns\n")
	assert.False(t, result.RequiresRestart)
	assert.Empty(t, result.Reasons)
}

func TestRestartDetector_DeviceChange(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("device = vpns\n", "device = tun0\n")
	assert.True(t, result.RequiresRestart)
	require.Len(t, result.Reasons, 1)
	assert.Contains(t, result.Reasons[0], "device")
}

func TestRestartDetector_MultipleRestartChanges(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("tcp-port = 443\nudp-port = 443\ndevice = vpns\n", "tcp-port = 8443\nudp-port = 8443\ndevice = tun0\n")
	assert.True(t, result.RequiresRestart)
	assert.Len(t, result.Reasons, 3)
}

func TestRestartDetector_DirectiveAdded(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("dns = 8.8.8.8\n", "dns = 8.8.8.8\nlisten-host = 0.0.0.0\n")
	assert.True(t, result.RequiresRestart)
	require.Len(t, result.Reasons, 1)
	assert.Contains(t, result.Reasons[0], "listen-host")
	assert.Contains(t, result.Reasons[0], "added")
}

func TestRestartDetector_DirectiveRemoved(t *testing.T) {
	det := NewRestartDetector()
	result := det.Detect("listen-host = 0.0.0.0\ndns = 8.8.8.8\n", "dns = 8.8.8.8\n")
	assert.True(t, result.RequiresRestart)
	require.Len(t, result.Reasons, 1)
	assert.Contains(t, result.Reasons[0], "listen-host")
	assert.Contains(t, result.Reasons[0], "removed")
}
