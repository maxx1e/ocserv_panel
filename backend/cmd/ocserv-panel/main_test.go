package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/monitoring"
)

func TestEnvOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		setValue *string
		fallback string
		want     string
	}{
		{name: "unset uses fallback", fallback: "fallback", want: "fallback"},
		{name: "empty uses fallback", setValue: stringPtr(""), fallback: "fallback", want: "fallback"},
		{name: "non-empty overrides fallback", setValue: stringPtr("configured"), fallback: "fallback", want: "configured"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "OCSERV_PANEL_TEST_ENV_OR_DEFAULT"
			if tt.setValue == nil {
				t.Setenv(key, "temporary")
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("unset env: %v", err)
				}
			} else {
				t.Setenv(key, *tt.setValue)
			}

			if got := envOrDefault(key, tt.fallback); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestTSDBPathFromEnv(t *testing.T) {
	t.Run("defaults to local data metrics path", func(t *testing.T) {
		t.Setenv("OCSERV_PANEL_TSDB_PATH", "")

		got := tsdbPathFromEnv()
		want := filepath.Join("data", "metrics")
		if got != want {
			t.Fatalf("tsdbPathFromEnv() = %q, want %q", got, want)
		}
	})

	t.Run("uses configured path", func(t *testing.T) {
		t.Setenv("OCSERV_PANEL_TSDB_PATH", filepath.Join("tmp", "panel-metrics"))

		got := tsdbPathFromEnv()
		want := filepath.Join("tmp", "panel-metrics")
		if got != want {
			t.Fatalf("tsdbPathFromEnv() = %q, want %q", got, want)
		}
	})
}

func TestConfiguredCollectors(t *testing.T) {
	t.Run("includes host collectors and auto-discovered interface by default", func(t *testing.T) {
		t.Setenv("OCSERV_PANEL_NET_IFACE", "")
		t.Setenv("OCSERV_PANEL_VPN_IFACE", "")

		collectors := configuredCollectors(fakeFileReader{})
		if len(collectors) < 2 {
			t.Fatalf("len(configuredCollectors()) = %d, want at least 2", len(collectors))
		}
	})

	t.Run("adds configured network and vpn interface collectors", func(t *testing.T) {
		t.Setenv("OCSERV_PANEL_NET_IFACE", "eth0")
		t.Setenv("OCSERV_PANEL_VPN_IFACE", "vpns0")
		reader := fakeFileReader{
			"/sys/class/net/eth0/statistics/rx_bytes":  []byte("10\n"),
			"/sys/class/net/eth0/statistics/tx_bytes":  []byte("20\n"),
			"/sys/class/net/vpns0/statistics/rx_bytes": []byte("30\n"),
			"/sys/class/net/vpns0/statistics/tx_bytes": []byte("40\n"),
		}

		collectors := configuredCollectors(reader)
		if len(collectors) != 4 {
			t.Fatalf("len(configuredCollectors()) = %d, want 4", len(collectors))
		}

		assertCollectorHasSample(t, collectors, metrics.MetricNetInterfaceRXBytes, "eth0")
		assertCollectorHasSample(t, collectors, metrics.MetricNetInterfaceTXBytes, "eth0")
		assertCollectorHasSample(t, collectors, metrics.MetricVPNInterfaceRXBytes, "vpns0")
		assertCollectorHasSample(t, collectors, metrics.MetricVPNInterfaceTXBytes, "vpns0")
	})
}

func TestCookieSecureFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty defaults true", value: "", want: true},
		{name: "true enables secure cookies", value: "true", want: true},
		{name: "one enables secure cookies", value: "1", want: true},
		{name: "false disables secure cookies", value: "false", want: false},
		{name: "zero disables secure cookies", value: "0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OCSERV_PANEL_COOKIE_SECURE", tt.value)
			if got := cookieSecureFromEnv(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestResolveMigrationsDirHonorsEnvAndFallbacks(t *testing.T) {
	t.Setenv("OCSERV_PANEL_MIGRATIONS_DIR", "/custom/migrations")
	if got := resolveMigrationsDir(); got != "/custom/migrations" {
		t.Fatalf("expected env migrations dir, got %q", got)
	}

	t.Setenv("OCSERV_PANEL_MIGRATIONS_DIR", "")
	temp := t.TempDir()
	if err := os.Mkdir(filepath.Join(temp, "migrations"), 0o755); err != nil {
		t.Fatalf("make migrations dir: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}

	if got := resolveMigrationsDir(); got != "migrations" {
		t.Fatalf("expected local migrations fallback, got %q", got)
	}
}

type fakeFileReader map[string][]byte

var _ monitoring.FileReader = fakeFileReader{}

func (r fakeFileReader) ReadFile(path string) ([]byte, error) {
	data := r[path]

	return data, nil
}

func assertCollectorHasSample(
	t *testing.T,
	collectors []monitoring.Collector,
	name metrics.MetricName,
	iface string,
) {
	t.Helper()

	for _, collector := range collectors {
		result := collector.Collect(context.Background(), time.Unix(100, 0).UTC())
		if result.Status != monitoring.CollectStatusAvailable {
			continue
		}

		for _, sample := range result.Samples {
			if sample.Name != name {
				continue
			}
			if sample.Labels["interface"] != iface {
				t.Fatalf("sample %q interface label = %q, want %q", name, sample.Labels["interface"], iface)
			}

			return
		}
	}

	t.Fatalf("configured collectors did not emit sample %q for interface %q", name, iface)
}

func stringPtr(value string) *string {
	return &value
}
