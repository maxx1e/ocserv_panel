package monitoring

import (
	"fmt"
	"os"
	"strings"
)

// DiscoverDefaultInterface attempts to find the default network interface
// by reading /proc/net/route. It returns the interface name of the first
// default route found, or an empty string if none is found.
//
// The function looks for routes with destination 00000000 (0.0.0.0),
// which indicates a default route. If multiple default routes exist,
// the first one found is returned.
//
// On non-Linux systems where /proc/net/route does not exist, this
// function returns an error.
func DiscoverDefaultInterface(opts ...DefaultInterfaceOption) (string, error) {
	cfg := defaultInterfaceConfig{
		routePath: "/proc/net/route",
		readFile:  os.ReadFile,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	data, err := cfg.readFile(cfg.routePath)
	if err != nil {
		return "", fmt.Errorf("read route file: %w", err)
	}

	return parseDefaultInterface(string(data))
}

type defaultInterfaceConfig struct {
	routePath string
	readFile  func(string) ([]byte, error)
}

// DefaultInterfaceOption configures default interface discovery.
type DefaultInterfaceOption func(*defaultInterfaceConfig)

// WithDefaultRoutePath sets the path to the route file (for testing).
func WithDefaultRoutePath(path string) DefaultInterfaceOption {
	return func(c *defaultInterfaceConfig) { c.routePath = path }
}

// WithDefaultReadFile sets the file reader function (for testing).
func WithDefaultReadFile(fn func(string) ([]byte, error)) DefaultInterfaceOption {
	return func(c *defaultInterfaceConfig) { c.readFile = fn }
}

// parseDefaultInterface parses /proc/net/route content and returns the
// interface name of the first default route.
//
// Format of /proc/net/route:
//
//	Iface   Destination     Gateway         Flags   RefCnt  Use     Metric  Mask            MTU     Window  IRTT
//	eth0    00000000        0100A8C0        0003    0       0       100     00000000        0       0       0
func parseDefaultInterface(data string) (string, error) {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return "", fmt.Errorf("route file is empty")
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		// Only a header line is present; no routes to inspect.
		// This is valid on systems with no configured interfaces.
		return "", nil
	}

	// Skip header line (index 0).
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		iface := fields[0]
		destination := fields[1]

		// Default route has destination 00000000 (0.0.0.0).
		if destination == "00000000" && iface != "" {
			return iface, nil
		}
	}

	return "", nil
}
