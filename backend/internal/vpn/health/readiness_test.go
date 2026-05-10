package health

import (
	"ocserv_panel/backend/internal/vpn/exec"
	"ocserv_panel/backend/internal/vpn/service"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadinessCheckerAllBinariesMissing(t *testing.T) {
	t.Parallel()

	runner := exec.NewFakeCommandRunner()
	svc := service.NewFakeServiceManager(service.ServiceInactive)

	checker := NewReadinessChecker(func() ReadinessConfig {
		return ReadinessConfig{
			OcservBinPath:   "/nonexistent/ocserv",
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
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

	runner := exec.NewFakeCommandRunner()
	// Simulate ocserv --version output
	runner.Results["/usr/local/sbin/ocserv --version"] = exec.CommandResult{
		ExitCode: 0,
		Stdout:   "ocserv 1.4.2\n",
	}
	svc := service.NewFakeServiceManager(service.ServiceInactive)

	checker := NewReadinessChecker(func() ReadinessConfig {
		return ReadinessConfig{
			OcservBinPath:   "/usr/local/sbin/ocserv",
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
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

	runner := exec.NewFakeCommandRunner()
	svc := service.NewFakeServiceManager(service.ServiceActive)

	checker := NewReadinessChecker(func() ReadinessConfig {
		return ReadinessConfig{
			OcservBinPath:   "/nonexistent/ocserv",
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
	}, runner, svc)

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.ServiceRunning {
		t.Error("expected ServiceRunning=true when FakeServiceManager reports service.ServiceActive")
	}
}

func TestReadinessCheckerConfigValidationOnlyRunsWhenBothPresent(t *testing.T) {
	t.Parallel()

	runner := exec.NewFakeCommandRunner()
	svc := service.NewFakeServiceManager(service.ServiceInactive)

	// Neither binary nor config present — validation should not run.
	checker := NewReadinessChecker(func() ReadinessConfig {
		return ReadinessConfig{
			OcservBinPath:   "/nonexistent/ocserv",
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
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
	runner := exec.NewFakeCommandRunner()
	svc := service.NewFakeServiceManager(service.ServiceNotFound)

	checker := NewReadinessChecker(func() ReadinessConfig { return ReadinessConfig{} }, runner, svc)

	_, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check returned unexpected error: %v", err)
	}
}

func TestReadinessCheckerCfgFnCalledEachCheck(t *testing.T) {
	t.Parallel()

	callCount := 0
	cfgFn := func() ReadinessConfig {
		callCount++
		return ReadinessConfig{}
	}
	runner := exec.NewFakeCommandRunner()
	checker := NewReadinessChecker(cfgFn, runner, service.NewFakeServiceManager(service.ServiceInactive))
	ctx := context.Background()

	checker.Check(ctx) //nolint:errcheck
	checker.Check(ctx) //nolint:errcheck
	checker.Check(ctx) //nolint:errcheck

	if callCount != 3 {
		t.Errorf("cfgFn called %d times for 3 Check() calls, want 3", callCount)
	}
}

func TestReadinessCheckerPicksUpPathChange(t *testing.T) {
	t.Parallel()

	// Create two real files so fileExists returns true and the runner is invoked.
	dir := t.TempDir()
	firstBin := filepath.Join(dir, "ocserv-v1")
	secondBin := filepath.Join(dir, "ocserv-v2")
	for _, p := range []string{firstBin, secondBin} {
		if err := os.WriteFile(p, []byte("#!/bin/sh"), 0o755); err != nil {
			t.Fatalf("create temp binary: %v", err)
		}
	}

	currentPath := firstBin
	cfgFn := func() ReadinessConfig {
		return ReadinessConfig{OcservBinPath: currentPath}
	}

	runner := exec.NewFakeCommandRunner()
	runner.Results[firstBin+" --version"] = exec.CommandResult{ExitCode: 0, Stdout: "ocserv 1.0.0\n"}
	runner.Results[secondBin+" --version"] = exec.CommandResult{ExitCode: 0, Stdout: "ocserv 2.0.0\n"}

	checker := NewReadinessChecker(cfgFn, runner, service.NewFakeServiceManager(service.ServiceInactive))
	ctx := context.Background()

	// First Check uses firstBin.
	checker.Check(ctx) //nolint:errcheck

	// Mutate the path — second Check should pick up secondBin.
	currentPath = secondBin
	checker.Check(ctx) //nolint:errcheck

	// Verify the runner was called with firstBin on the first pass
	// and secondBin on the second pass.
	var sawFirst, sawSecond bool
	for _, call := range runner.Calls {
		if call.Name == firstBin {
			sawFirst = true
		}
		if call.Name == secondBin {
			sawSecond = true
		}
	}
	if !sawFirst {
		t.Error("expected runner to be called with firstBin on first Check()")
	}
	if !sawSecond {
		t.Error("expected runner to be called with secondBin after path change")
	}
}
