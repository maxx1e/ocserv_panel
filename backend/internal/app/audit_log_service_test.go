package app

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/rbac"
)

func TestAuditLogService_ReadLogs_Success(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, &logs.ReadResult{
		Lines:   []logs.LogLine{{Level: "info", Message: "test"}},
		Total:   1,
		HasMore: false,
	}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	result, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "panel" {
		t.Errorf("source = %q, want panel", result.Source)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(result.Lines))
	}
}

func TestAuditLogService_ReadLogs_Unauthenticated(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrUnauthenticated {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestAuditLogService_ReadLogs_PermissionDenied(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.Authorizer{},
	)
	principal := auth.Principal{ID: 1, Username: "nobody", Roles: []string{"invalid"}}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrPermissionDenied {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestAuditLogService_ReadLogs_InvalidSource(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	_, err := svc.ReadLogs(context.Background(), principal, "../../etc/passwd", AuditLogQuery{})
	if err != ErrInvalidInput {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAuditLogService_ReadLogs_Unavailable(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(false, nil, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrLogSourceUnavailable {
		t.Errorf("err = %v, want ErrLogSourceUnavailable", err)
	}
}

func TestAuditLogService_ReadLogs_JournalFallback(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(true, &logs.ReadResult{
		Lines:   []logs.LogLine{{Level: "info", Message: "from journal"}},
		Total:   1,
		HasMore: false,
	}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	result, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0].Message != "from journal" {
		t.Errorf("expected journal fallback result, got %+v", result.Lines)
	}
}

func TestAuditLogService_GetSources(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, nil, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader, "ocserv": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("got %d sources, want 2", len(sources))
	}
}

func TestAuditLogService_ExportLogs_PermissionDenied(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	// viewer role cannot export
	principal := auth.Principal{ID: 1, Username: "viewer", Roles: []string{"viewer"}}
	_, err := svc.ExportLogs(context.Background(), principal, "panel", AuditLogQuery{}, "json")
	if err != ErrPermissionDenied {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestAuditLogService_ReadLogs_TimeValidation(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, &logs.ReadResult{Lines: nil, Total: 0}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	// from > to is invalid
	query := AuditLogQuery{
		From: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", query)
	if err != ErrInvalidInput {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAuditLogService_GetSources_Reason_Unavailable(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(false, nil, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile, "ocserv": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal, "ocserv": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, src := range sources {
		if src.Available {
			t.Errorf("source %s should not be available", src.Name)
		}
		if src.Reason != "service_not_running" {
			t.Errorf("source %s reason = %q, want service_not_running", src.Name, src.Reason)
		}
	}
}

func TestAuditLogService_GetSources_Reason_Available(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(true, &logs.ReadResult{}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile, "ocserv": fakeFile},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, src := range sources {
		if !src.Available {
			t.Errorf("source %s should be available", src.Name)
		}
		if src.Reason != "" {
			t.Errorf("source %s reason = %q, want empty", src.Name, src.Reason)
		}
	}
}
