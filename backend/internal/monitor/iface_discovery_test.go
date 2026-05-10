package monitor

import (
	"errors"
	"testing"
)

func TestDiscoverDefaultInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "single default route",
			data: "Iface\tDestination\tGateway\t\tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
				"eth0\t00000000\t0100A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
				"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n",
			want: "eth0",
		},
		{
			name: "multiple interfaces with default route first",
			data: "Iface\tDestination\tGateway\t\tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
				"enp0s3\t00000000\t0100A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
				"docker0\t0001A8C0\t00000000\t0001\t0\t0\t0\t00FFFF00\t0\t0\t0\n",
			want: "enp0s3",
		},
		{
			name: "no default route",
			data: "Iface\tDestination\tGateway\t\tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
				"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n",
			want: "",
		},
		{
			name:    "empty data",
			data:    "",
			wantErr: true,
		},
		{
			name: "header only",
			data: "Iface\tDestination\tGateway\t\tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n",
			want: "",
		},
		{
			name: "default route on non-first line",
			data: "Iface\tDestination\tGateway\t\tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
				"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n" +
				"wlp2s0\t00000000\t0100A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n",
			want: "wlp2s0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			iface, err := DiscoverDefaultInterface(
				WithDefaultReadFile(func(_ string) ([]byte, error) {
					return []byte(tt.data), nil
				}),
			)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DiscoverDefaultInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			if iface != tt.want {
				t.Fatalf("DiscoverDefaultInterface() = %q, want %q", iface, tt.want)
			}
		})
	}
}

func TestDiscoverDefaultInterface_ReadError(t *testing.T) {
	t.Parallel()

	_, err := DiscoverDefaultInterface(
		WithDefaultReadFile(func(_ string) ([]byte, error) {
			return nil, errors.New("file not found")
		}),
	)
	if err == nil {
		t.Fatal("expected error when route file cannot be read")
	}
}

func TestParseDefaultInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "parses default route",
			data: "Iface\tDestination\tGateway\neth0\t00000000\t0100A8C0\n",
			want: "eth0",
		},
		{
			name: "parses without trailing newline",
			data: "Iface\tDestination\tGateway\neth0\t00000000\t0100A8C0",
			want: "eth0",
		},
		{
			name: "no data lines",
			data: "Iface\tDestination\n",
			want: "",
		},
		{
			name:    "empty string",
			data:    "",
			wantErr: true,
		},
		{
			name: "whitespace-only destination not matched",
			data: "Iface\tDestination\nlo\t        \n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			iface, err := parseDefaultInterface(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDefaultInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			if iface != tt.want {
				t.Fatalf("parseDefaultInterface() = %q, want %q", iface, tt.want)
			}
		})
	}
}
