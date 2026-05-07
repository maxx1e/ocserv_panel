package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

func TestHealthServiceSystemHealth_HappyPath(t *testing.T) {
	t.Parallel()

	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig {
			return ocserv.ReadinessConfig{
				OcservBinPath: "/nonexistent/ocserv",
				ConfigPath:    "/nonexistent/ocserv.conf",
				SocketPath:    "/nonexistent/ocserv.sock",
			}
		},
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceActive),
	)

	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(checker, stubAdminCounter(2), cfgFn, rbac.NewDefaultAuthorizer(), "")

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Panel.BackendStatus != "ok" {
		t.Errorf("expected panel.backendStatus=ok, got %q", result.Panel.BackendStatus)
	}
	if result.Panel.WebuiUserCount != 2 {
		t.Errorf("expected panel.webuiUserCount=2, got %d", result.Panel.WebuiUserCount)
	}
}

func TestHealthServiceSystemHealth_Unauthenticated(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer(), "")

	_, err := svc.SystemHealth(context.Background(), auth.Principal{})
	if err != app.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestHealthServiceSystemHealth_PermissionDenied(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer(), "")

	// unknown_role has no permissions
	principal2 := auth.Principal{ID: 3, Username: "noperm", Roles: []string{"unknown_role"}}
	_, err := svc.SystemHealth(context.Background(), principal2)
	if err != app.ErrPermissionDenied {
		t.Fatalf("expected ErrPermissionDenied for unknown role, got %v", err)
	}
	// viewer should succeed (has PermissionReadDashboard)
	viewer := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	_, err2 := svc.SystemHealth(context.Background(), viewer)
	if err2 != nil {
		t.Fatalf("viewer should have read dashboard perm: %v", err2)
	}
}

func TestHealthServiceSystemHealth_StorageFields(t *testing.T) {
	t.Parallel()

	// Create a temp dir to simulate a TSDB directory with files.
	tsdbDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tsdbDir, "data.bin"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.Installation.DataDir = t.TempDir()
	// Create a fake SQLite DB file matching the DSN-derived path.
	dbPath := filepath.Join(cfg.Installation.DataDir, "ocserv-panel.db")
	if err := os.WriteFile(dbPath, make([]byte, 2048), 0644); err != nil {
		t.Fatal(err)
	}
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(nil, stubAdminCounter(0), cfgFn, rbac.NewDefaultAuthorizer(), tsdbDir)

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SQLite checks
	if result.Storage.SQLite.Engine != "sqlite" {
		t.Errorf("expected sqlite engine, got %q", result.Storage.SQLite.Engine)
	}
	if result.Storage.SQLite.SizeBytes != 2048 {
		t.Errorf("expected sqlite size=2048, got %d", result.Storage.SQLite.SizeBytes)
	}
	if result.Storage.SQLite.Status != "healthy" {
		t.Errorf("expected sqlite status=healthy, got %q", result.Storage.SQLite.Status)
	}

	// TSDB checks
	if result.Storage.TSDB.Engine != "tsdb" {
		t.Errorf("expected tsdb engine, got %q", result.Storage.TSDB.Engine)
	}
	if result.Storage.TSDB.SizeBytes != 1024 {
		t.Errorf("expected tsdb size=1024, got %d", result.Storage.TSDB.SizeBytes)
	}
	if result.Storage.TSDB.Status != "healthy" {
		t.Errorf("expected tsdb status=healthy, got %q", result.Storage.TSDB.Status)
	}
}

func TestStorageStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		size      int64
		warn      int64
		crit      int64
		expected  string
	}{
		{name: "below warning", size: 100, warn: 500, crit: 1000, expected: "healthy"},
		{name: "at warning threshold", size: 500, warn: 500, crit: 1000, expected: "warning"},
		{name: "above warning below critical", size: 750, warn: 500, crit: 1000, expected: "warning"},
		{name: "at critical threshold", size: 1000, warn: 500, crit: 1000, expected: "critical"},
		{name: "above critical", size: 2000, warn: 500, crit: 1000, expected: "critical"},
		{name: "zero size", size: 0, warn: 500, crit: 1000, expected: "healthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.StorageStatusForTest(tt.size, tt.warn, tt.crit)
			if got != tt.expected {
				t.Errorf("storageStatus(%d, %d, %d) = %q, want %q", tt.size, tt.warn, tt.crit, got, tt.expected)
			}
		})
	}
}

func TestMaskPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		segments int
		expected string
	}{
		{name: "single segment from long path", path: "/var/lib/ocserv-panel/panel.db", segments: 1, expected: "…/panel.db"},
		{name: "two segments from long path", path: "/var/lib/ocserv-panel/data/metrics", segments: 2, expected: "…/data/metrics"},
		{name: "path shorter than segments", path: "panel.db", segments: 2, expected: "panel.db"},
		{name: "empty path", path: "", segments: 1, expected: ""},
		{name: "exact segment count", path: "data/metrics", segments: 2, expected: "data/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.MaskPathForTest(tt.path, tt.segments)
			if got != tt.expected {
				t.Errorf("maskPath(%q, %d) = %q, want %q", tt.path, tt.segments, got, tt.expected)
			}
		})
	}
}

func TestFileSize(t *testing.T) {
	t.Parallel()

	t.Run("existing file", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "test.db")
		data := make([]byte, 4096)
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			t.Fatal(err)
		}
		got := app.FileSizeForTest(tmp)
		if got != 4096 {
			t.Errorf("fileSize(%q) = %d, want 4096", tmp, got)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		got := app.FileSizeForTest("/nonexistent/path/file.db")
		if got != 0 {
			t.Errorf("fileSize(nonexistent) = %d, want 0", got)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		got := app.FileSizeForTest("")
		if got != 0 {
			t.Errorf("fileSize(\"\") = %d, want 0", got)
		}
	})

	t.Run("directory returns zero", func(t *testing.T) {
		dir := t.TempDir()
		got := app.FileSizeForTest(dir)
		if got != 0 {
			t.Errorf("fileSize(dir) = %d, want 0", got)
		}
	})
}

func TestDirSize(t *testing.T) {
	t.Parallel()

	t.Run("directory with files", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "a.bin"), make([]byte, 100), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.bin"), make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "c.bin"), make([]byte, 50), 0644); err != nil {
			t.Fatal(err)
		}

		got := app.DirSizeForTest(dir)
		if got != 350 {
			t.Errorf("dirSize = %d, want 350", got)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got := app.DirSizeForTest(dir)
		if got != 0 {
			t.Errorf("dirSize(empty dir) = %d, want 0", got)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		got := app.DirSizeForTest("/nonexistent/path/dir")
		if got != 0 {
			t.Errorf("dirSize(nonexistent) = %d, want 0", got)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		got := app.DirSizeForTest("")
		if got != 0 {
			t.Errorf("dirSize(\"\") = %d, want 0", got)
		}
	})
}

// stubAdminCounter is a minimal adminCounter for tests.
type stubAdminCounter int64

func (s stubAdminCounter) AdminCount(_ context.Context) (int64, error) {
	return int64(s), nil
}

func TestHealthServiceSystemHealth_PasswordFilePresent(t *testing.T) {
	t.Parallel()

	// Create a temp password file so readiness can find it.
	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	if err := os.WriteFile(passwdFile, []byte("user1:*:hash\n"), 0600); err != nil {
		t.Fatal(err)
	}

	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig {
			return ocserv.ReadinessConfig{
				OcservBinPath:    "/nonexistent/ocserv",
				ConfigPath:       "/nonexistent/ocserv.conf",
				SocketPath:       "/nonexistent/ocserv.sock",
				PasswordFilePath: passwdFile,
			}
		},
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceActive),
	)

	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(checker, stubAdminCounter(1), cfgFn, rbac.NewDefaultAuthorizer(), "")

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.VPNServer.PasswordFilePresent {
		t.Error("expected passwordFilePresent=true when file exists")
	}
}

func TestHealthServiceSystemHealth_PasswordFileMissing(t *testing.T) {
	t.Parallel()

	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig {
			return ocserv.ReadinessConfig{
				OcservBinPath:    "/nonexistent/ocserv",
				ConfigPath:       "/nonexistent/ocserv.conf",
				SocketPath:       "/nonexistent/ocserv.sock",
				PasswordFilePath: "/nonexistent/ocpasswd",
			}
		},
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceActive),
	)

	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(checker, stubAdminCounter(1), cfgFn, rbac.NewDefaultAuthorizer(), "")

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VPNServer.PasswordFilePresent {
		t.Error("expected passwordFilePresent=false when file doesn't exist")
	}
}
