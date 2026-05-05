package ocserv

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidatorMissingFile(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent/ocserv.conf")
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 1 || issues[0].Severity != IssueError {
		t.Fatalf("expected 1 error issue for missing file, got %v", issues)
	}
	if issues[0].Key != "config_file" {
		t.Fatalf("expected key=config_file, got %q", issues[0].Key)
	}
}

func TestConfigValidatorRelativeCertPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("server-cert = ../tests/certs/server-cert.pem\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == IssueWarning && issue.Key == "server-cert" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected warning for relative cert path, got %v", issues)
	}
}

func TestConfigValidatorMissingCertFileError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	// Absolute path that doesn't exist.
	if err := os.WriteFile(confPath, []byte("server-cert = /nonexistent/cert.pem\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasError := false
	for _, issue := range issues {
		if issue.Severity == IssueError && issue.Key == "server-cert" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatalf("expected error for missing cert file, got %v", issues)
	}
}

func TestConfigValidatorAuthWithMissingPasswd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("auth = \"plain[passwd=./sample.passwd]\"\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasInfo := false
	for _, issue := range issues {
		if issue.Severity == IssueInfo && issue.Key == "auth" {
			hasInfo = true
		}
	}
	if !hasInfo {
		t.Fatalf("expected info for missing passwd file, got %v", issues)
	}
}

func TestConfigValidatorAuthWithRelativePasswd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("auth = \"plain[passwd=./ocpasswd]\"\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == IssueWarning && issue.Key == "auth" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected warning for relative passwd path, got %v", issues)
	}
}

func TestConfigValidatorValidAbsoluteCertPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server-cert.pem")
	if err := os.WriteFile(certFile, []byte("cert"), 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("server-cert = "+certFile+"\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	for _, issue := range issues {
		if issue.Severity == IssueError && issue.Key == "server-cert" {
			t.Fatalf("unexpected error for valid cert path: %v", issue)
		}
	}
}

func TestConfigValidatorRelativeSocketPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("socket-file = ./ocserv-socket\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == IssueWarning && issue.Key == "socket-file" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected warning for relative socket path, got %v", issues)
	}
}

func TestConfigValidatorEmptyConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("# just comments\n\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues for comment-only config, got %v", issues)
	}
}

func TestParseConfigLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line   string
		key    string
		value  string
		wantOK bool
	}{
		{"server-cert = /etc/ocserv/certs/server-cert.pem", "server-cert", "/etc/ocserv/certs/server-cert.pem", true},
		{"auth = \"plain[passwd=./sample.passwd]\"", "auth", "plain[passwd=./sample.passwd]", true},
		{"device = vpns0", "device", "vpns0", true},
		{"# comment", "", "", false},
		{"", "", "", false},
		{"no-equals-sign", "", "", false},
	}

	for _, tt := range tests {
		key, value, ok := parseConfigLine(tt.line)
		if ok != tt.wantOK || key != tt.key || value != tt.value {
			t.Errorf("parseConfigLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.line, key, value, ok, tt.key, tt.value, tt.wantOK)
		}
	}
}

func TestConfigValidatorValidateContentPortRange(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent")
	issues, err := v.ValidateContent(context.Background(), "tcp-port = 99999\n")
	if err != nil {
		t.Fatalf("ValidateContent: %v", err)
	}
	for _, issue := range issues {
		if issue.Key == "tcp-port" && issue.Severity == IssueError {
			return
		}
	}
	t.Fatalf("expected error issue for out-of-range port, got %v", issues)
}

func TestConfigValidatorValidateContentInvalidCIDR(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent")
	issues, err := v.ValidateContent(context.Background(), "route = not-a-cidr\n")
	if err != nil {
		t.Fatalf("ValidateContent: %v", err)
	}
	for _, issue := range issues {
		if issue.Key == "route" && issue.Severity == IssueError {
			return
		}
	}
	t.Fatalf("expected error issue for invalid CIDR, got %v", issues)
}

func TestConfigValidatorValidateContentInvalidDNS(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent")
	issues, err := v.ValidateContent(context.Background(), "dns = not.an.ip\n")
	if err != nil {
		t.Fatalf("ValidateContent: %v", err)
	}
	for _, issue := range issues {
		if issue.Key == "dns" && issue.Severity == IssueWarning {
			return
		}
	}
	t.Fatalf("expected warning issue for invalid DNS IP, got %v", issues)
}

func TestConfigValidatorValidateContentValid(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent")
	content := "tcp-port = 443\nudp-port = 443\nroute = 10.0.0.0/8\ndns = 8.8.8.8\n"
	issues, err := v.ValidateContent(context.Background(), content)
	if err != nil {
		t.Fatalf("ValidateContent: %v", err)
	}
	for _, issue := range issues {
		if issue.Severity == IssueError {
			t.Fatalf("unexpected error issue for valid content: %v", issue)
		}
	}
}

func TestConfigValidatorValidateContentMalformedLine(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent")
	issues, err := v.ValidateContent(context.Background(), "this-line-has-no-equals\n")
	if err != nil {
		t.Fatalf("ValidateContent: %v", err)
	}
	for _, issue := range issues {
		if issue.Severity == IssueError {
			return
		}
	}
	t.Fatalf("expected error issue for malformed line, got %v", issues)
}
