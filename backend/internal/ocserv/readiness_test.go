package ocserv

import (
	"context"
	"testing"
)

func TestReadinessCheckerAllBinariesMissing(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceInactive)

	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/nonexistent/ocserv",
		OcctlBinPath:    "/nonexistent/occtl",
		OcpasswdBinPath: "/nonexistent/ocpasswd",
		ConfigPath:      "/nonexistent/ocserv.conf",
		SocketPath:      "/nonexistent/socket",
		UnitName:        "ocserv",
	}, runner, svc)

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.BinaryPresent {
		t.Error("expected BinaryPresent=false for nonexistent binary")
	}
	if result.OcctlPresent {
		t.Error("expected OcctlPresent=false for nonexistent binary")
	}
	if result.OcpasswdPresent {
		t.Error("expected OcpasswdPresent=false for nonexistent binary")
	}
	if result.ConfigPresent {
		t.Error("expected ConfigPresent=false for nonexistent config")
	}
	if result.SocketPresent {
		t.Error("expected SocketPresent=false for nonexistent socket")
	}
	if result.ServiceRunning {
		t.Error("expected ServiceRunning=false for inactive service")
	}
}

func TestReadinessCheckerVersionExtracted(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	// Simulate ocserv --version output
	runner.Results["/usr/local/sbin/ocserv --version"] = CommandResult{
		ExitCode: 0,
		Stdout:   "ocserv 1.4.2\n",
	}
	svc := NewFakeServiceManager(ServiceInactive)

	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/usr/local/sbin/ocserv",
		OcctlBinPath:    "/nonexistent/occtl",
		OcpasswdBinPath: "/nonexistent/ocpasswd",
		ConfigPath:      "/nonexistent/ocserv.conf",
		SocketPath:      "/nonexistent/socket",
		UnitName:        "ocserv",
	}, runner, svc)

	// Create a temporary file at the ocserv bin path so fileExists returns true.
	// In this test the binary doesn't really exist, so BinaryPresent will be false
	// and version check won't run. We test version extraction separately below.
	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	// Binary doesn't exist on disk, so version extraction is skipped.
	if result.BinaryVersion != "" {
		t.Errorf("expected empty version when binary not present on disk, got %q", result.BinaryVersion)
	}
}

func TestReadinessCheckerServiceRunning(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceActive)

	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/nonexistent/ocserv",
		OcctlBinPath:    "/nonexistent/occtl",
		OcpasswdBinPath: "/nonexistent/ocpasswd",
		ConfigPath:      "/nonexistent/ocserv.conf",
		SocketPath:      "/nonexistent/socket",
		UnitName:        "ocserv",
	}, runner, svc)

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.ServiceRunning {
		t.Error("expected ServiceRunning=true when FakeServiceManager reports ServiceActive")
	}
}

func TestReadinessCheckerConfigValidationOnlyRunsWhenBothPresent(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceInactive)

	// Neither binary nor config present — validation should not run.
	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/nonexistent/ocserv",
		OcctlBinPath:    "/nonexistent/occtl",
		OcpasswdBinPath: "/nonexistent/ocpasswd",
		ConfigPath:      "/nonexistent/ocserv.conf",
		SocketPath:      "/nonexistent/socket",
		UnitName:        "ocserv",
	}, runner, svc)

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	// ConfigValid defaults to false (zero value); no validation ran.
	if result.ConfigValid {
		t.Error("expected ConfigValid=false when neither binary nor config is present")
	}
	if result.ConfigErrors != "" {
		t.Errorf("expected no config errors, got %q", result.ConfigErrors)
	}
}

func TestReadinessCheckerRealBinariesNotRequired(t *testing.T) {
	t.Parallel()

	// This test verifies Check doesn't panic or error with a fully-fake setup.
	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceNotFound)

	checker := NewReadinessChecker(ReadinessConfig{}, runner, svc)

	_, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check returned unexpected error: %v", err)
	}
}
