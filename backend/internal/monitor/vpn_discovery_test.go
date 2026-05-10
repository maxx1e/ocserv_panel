package monitor

import (
	"os"
	"testing"
)

// fakeFileReaderForDiscovery is a FileReader backed by a string map.
// Each file maps to a single content string.
type fakeFileReaderForDiscovery struct {
	files map[string]string
}

func (r *fakeFileReaderForDiscovery) ReadFile(path string) ([]byte, error) {
	content, ok := r.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(content), nil
}

// fakeDirEntry implements os.DirEntry for testing ReadDir.
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return e.isDir }
func (e fakeDirEntry) Type() os.FileMode           { return 0 }
func (e fakeDirEntry) Info() (os.FileInfo, error)  { return nil, nil }

func TestDiscoverVPNInterfaceFromOcservConfig(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/ocserv/ocserv.conf": "device = tun0\ntcp-port = 443\n",
	}
	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: files}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{fakeDirEntry{name: "tun0"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q", iface, "tun0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}

func TestDiscoverVPNInterfaceFromSysFS(t *testing.T) {
	t.Parallel()

	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{fakeDirEntry{name: "tun0"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q", iface, "tun0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}

func TestDiscoverVPNInterfaceNoConfigNoInterfaces(t *testing.T) {
	t.Parallel()

	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return nil, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "" {
		t.Fatalf("iface = %q, want empty string", iface)
	}
	if source != "" {
		t.Fatalf("source = %q, want empty string", source)
	}
}

func TestDiscoverVPNInterfacePrefersOcservConfig(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/ocserv/ocserv.conf": "device = vpn0\n",
	}
	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: files}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "vpn0"},
				fakeDirEntry{name: "tun0"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpn0" {
		t.Fatalf("iface = %q, want %q (ocserv config should take priority)", iface, "vpn0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}

func TestDiscoverVPNInterfaceMultipleTunFallsBackToTun0(t *testing.T) {
	t.Parallel()

	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "tun0"},
				fakeDirEntry{name: "tun1"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q (tun0 preferred when multiple)", iface, "tun0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}

func TestDiscoverVPNInterfaceVpnsPrefixFromSysFS(t *testing.T) {
	t.Parallel()

	iface, source, err := DiscoverVPNInterface(
		WithOcservConfigPath("/nonexistent"),
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "eth0"},
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "lo"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpns0" {
		t.Fatalf("iface = %q, want %q", iface, "vpns0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}

func TestDiscoverVPNInterfaceConfigDeviceBaseNameResolvesToNumbered(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/ocserv/ocserv.conf": "device = vpns\n",
	}
	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReaderForDiscovery{files: files}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "eth0"},
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "lo"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpns0" {
		t.Fatalf("iface = %q, want %q (config base name should resolve to first numbered instance)", iface, "vpns0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}

func TestDiscoverAllVPNInterfacesFromSysFS(t *testing.T) {
	t.Parallel()

	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "eth0"},
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "lo"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
	if ifaces[0] != "vpns0" || ifaces[1] != "vpns1" {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
}

func TestDiscoverAllVPNInterfacesFromOcservConfig(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/ocserv/ocserv.conf": "device = vpns\n",
	}
	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: files}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "eth0"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
}

func TestDiscoverAllVPNInterfacesEmpty(t *testing.T) {
	t.Parallel()

	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{fakeDirEntry{name: "eth0"}, fakeDirEntry{name: "lo"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 0 {
		t.Fatalf("ifaces = %v, want empty", ifaces)
	}
}

func TestResolveAllInterfaceNames(t *testing.T) {
	t.Parallel()

	cfg := vpnDiscoveryConfig{
		sysNetPath: "/sys/class/net",
		readDir: func(string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "vpns2"},
				fakeDirEntry{name: "eth0"},
			}, nil
		},
	}
	got := resolveAllInterfaceNames(cfg, "vpns")
	if len(got) != 3 {
		t.Fatalf("resolveAllInterfaceNames = %v, want [vpns0, vpns1, vpns2]", got)
	}
}
