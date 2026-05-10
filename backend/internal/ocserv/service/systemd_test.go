package service_test

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/ocserv/cmd"
	"ocserv_panel/backend/internal/ocserv/service"
)

// FakeServiceManager tests

func TestFakeServiceManagerDefaultStatus(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceActive)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != service.ServiceActive {
		t.Fatalf("expected ServiceActive, got %d", status)
	}
}

func TestFakeServiceManagerStart(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceInactive)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != service.ServiceActive {
		t.Fatalf("expected ServiceActive after start, got %d", status)
	}
	if svc.StartCount() != 1 {
		t.Fatalf("expected 1 start, got %d", svc.StartCount())
	}
}

func TestFakeServiceManagerStop(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceActive)
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != service.ServiceInactive {
		t.Fatalf("expected ServiceInactive after stop, got %d", status)
	}
	if svc.StopCount() != 1 {
		t.Fatalf("expected 1 stop, got %d", svc.StopCount())
	}
}

func TestFakeServiceManagerRestart(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceInactive)
	if err := svc.Restart(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != service.ServiceActive {
		t.Fatalf("expected ServiceActive after restart, got %d", status)
	}
	if svc.RestartCount() != 1 {
		t.Fatalf("expected 1 restart, got %d", svc.RestartCount())
	}
}

func TestFakeServiceManagerIsEnabled(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceActive)
	enabled, err := svc.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("is-enabled: %v", err)
	}
	if !enabled {
		t.Fatal("expected enabled to be true by default")
	}
}

func TestFakeServiceManagerSetError(t *testing.T) {
	t.Parallel()
	svc := service.NewFakeServiceManager(service.ServiceActive)
	svc.SetError(errors.New("systemctl failed"))
	_, err := svc.Status(context.Background())
	if err == nil {
		t.Fatal("expected error from status")
	}
}

// SystemdManager tests (using FakeCommandRunner)

func TestSystemdManagerStatusActive(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{
		ExitCode: 0,
		Stdout:   "● ocserv.service - ocserv\n   Active: active (running) since Mon 2026-01-01 00:00:00 UTC\n",
	}
	mgr := service.NewSystemdManager("ocserv", runner)
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != service.ServiceActive {
		t.Fatalf("expected ServiceActive, got %d", status)
	}
}

func TestSystemdManagerStatusInactive(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{
		ExitCode: 3,
		Stdout:   "● ocserv.service - ocserv\n   Active: inactive (dead)\n",
	}
	mgr := service.NewSystemdManager("ocserv", runner)
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != service.ServiceInactive {
		t.Fatalf("expected ServiceInactive, got %d", status)
	}
}

func TestSystemdManagerStatusFailed(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{
		ExitCode: 3,
		Stdout:   "● ocserv.service - ocserv\n   Active: failed (Result: exit-code)\n",
	}
	mgr := service.NewSystemdManager("ocserv", runner)
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != service.ServiceFailed {
		t.Fatalf("expected ServiceFailed, got %d", status)
	}
}

func TestSystemdManagerStart(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 0}
	mgr := service.NewSystemdManager("ocserv", runner)
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
}

func TestSystemdManagerStartFailsOnNonZeroExit(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 1, Stderr: "Job failed."}
	mgr := service.NewSystemdManager("ocserv", runner)
	if err := mgr.Start(context.Background()); err == nil {
		t.Fatal("expected start to fail")
	}
}

func TestSystemdManagerStop(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 0}
	mgr := service.NewSystemdManager("ocserv", runner)
	if err := mgr.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestSystemdManagerRestart(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 0}
	mgr := service.NewSystemdManager("ocserv", runner)
	if err := mgr.Restart(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
}

func TestSystemdManagerIsEnabled(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 0, Stdout: "enabled\n"}
	mgr := service.NewSystemdManager("ocserv", runner)
	enabled, err := mgr.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("is-enabled: %v", err)
	}
	if !enabled {
		t.Fatal("expected enabled")
	}
}

func TestSystemdManagerIsEnabledWhenDisabled(t *testing.T) {
	t.Parallel()
	runner := cmd.NewFakeCommandRunner()
	runner.Results["systemctl"] = cmd.CommandResult{ExitCode: 1, Stdout: "disabled\n"}
	mgr := service.NewSystemdManager("ocserv", runner)
	enabled, err := mgr.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("is-enabled: %v", err)
	}
	if enabled {
		t.Fatal("expected not enabled")
	}
}
