package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)

func testLogTailCfg(ocservLog, panelLog string, enabled bool) func() config.Config {
	cfg := config.Defaults()
	cfg.Installation.OcservLogPath = ocservLog
	cfg.Installation.PanelLogPath = panelLog
	cfg.Monitoring.LogFileReadEnabled = enabled
	return func() config.Config { return cfg }
}

func ownerPrincipal() auth.Principal {
	return auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
}

func TestLogTailService_Disabled(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", false),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Enabled {
		t.Errorf("expected enabled=false")
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected no lines when disabled")
	}
}

func TestLogTailService_FileNotFound(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/nonexistent/ocserv.log", "/nonexistent/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Enabled {
		t.Errorf("expected enabled=true when LogFileReadEnabled=true")
	}
	if result.Present {
		t.Errorf("expected present=false for nonexistent file")
	}
}

func TestLogTailService_NormalTail(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	content := strings.Join(lines, "\n") + "\n"

	reader := app.FakeLogFileReader{"/var/log/ocserv.log": []byte(content)}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader,
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Enabled || !result.Present {
		t.Errorf("expected enabled=true, present=true")
	}
	if len(result.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(result.Lines), result.Lines)
	}
	if result.Lines[0] != "line3" || result.Lines[2] != "line5" {
		t.Errorf("expected last 3 lines: %v", result.Lines)
	}
}

func TestLogTailService_LinesCap(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line\n")
	}

	reader := app.FakeLogFileReader{"/var/log/ocserv.log": []byte(sb.String())}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader,
		rbac.NewDefaultAuthorizer(),
	)

	// Request 200 lines — hard cap should clamp to 50
	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) > 50 {
		t.Errorf("expected max 50 lines, got %d", len(result.Lines))
	}
}

func TestLogTailService_ReadError(t *testing.T) {
	t.Parallel()

	reader2 := app.ErrorLogFileReader{Path: "/var/log/ocserv.log", Err: errors.New("permission denied")}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader2,
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error (should be in result, not returned): %v", err)
	}
	if result.Error == "" {
		t.Errorf("expected error field to be set on read failure")
	}
}

func TestLogTailService_InvalidSource(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	_, err := svc.TailLog(context.Background(), ownerPrincipal(), "invalid", 25)
	if err == nil {
		t.Errorf("expected error for invalid source")
	}
}

func TestLogTailService_Unauthenticated(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	_, err := svc.TailLog(context.Background(), auth.Principal{}, "ocserv", 25)
	if err != app.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
