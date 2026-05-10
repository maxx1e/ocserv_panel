package monitoring

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DiscoverVPNInterface attempts to find the VPN interface name by inspecting
// the ocserv configuration and scanning /sys/class/net for typical VPN interfaces.
//
// It returns (interfaceName, source, error). An empty interfaceName indicates
// no VPN interface was found; source is "ocserv-config" or "sysfs-scan" when
// an interface is found, and empty otherwise.
func DiscoverVPNInterface(opts ...VPNDiscoveryOption) (string, string, error) {
	cfg := vpnDiscoveryConfig{
		reader:     OSReader{},
		configPath: "/etc/ocserv/ocserv.conf",
		sysNetPath: "/sys/class/net",
		readDir:    os.ReadDir,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Strategy 1: Parse ocserv config for the "device" directive.
	iface, err := discoverFromOcservConfig(cfg)
	if err == nil && iface != "" {
		resolved := resolveInterfaceName(cfg, iface)
		if resolved != "" {
			return resolved, "ocserv-config", nil
		}
	}

	// Strategy 2: Scan /sys/class/net/ for VPN interface patterns.
	iface, err = discoverFromSysFS(cfg)
	if err == nil && iface != "" {
		return iface, "sysfs-scan", nil
	}

	// No VPN interface found.
	return "", "", nil
}

// vpnDiscoveryConfig holds all configurable parameters for VPN discovery.
type vpnDiscoveryConfig struct {
	reader     FileReader
	configPath string
	sysNetPath string
	readDir    func(dirname string) ([]os.DirEntry, error)
}

// VPNDiscoveryOption configures VPN interface discovery.
type VPNDiscoveryOption func(*vpnDiscoveryConfig)

// WithFileReader sets the FileReader used to read the ocserv configuration file.
func WithFileReader(reader FileReader) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.reader = reader }
}

// WithOcservConfigPath sets the path to the ocserv configuration file.
func WithOcservConfigPath(path string) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.configPath = path }
}

// WithSysNetPath sets the path used as /sys/class/net for interface discovery.
func WithSysNetPath(path string) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.sysNetPath = path }
}

// WithReadDirFunc sets the function used to read a directory (for testing).
func WithReadDirFunc(fn func(dirname string) ([]os.DirEntry, error)) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.readDir = fn }
}

// discoverFromOcservConfig reads the ocserv configuration file and returns the
// value of the first "device" directive, ignoring comment lines.
func discoverFromOcservConfig(cfg vpnDiscoveryConfig) (string, error) {
	data, err := cfg.reader.ReadFile(cfg.configPath)
	if err != nil {
		return "", fmt.Errorf("read ocserv config: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "device") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if value != "" {
					return value, nil
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan ocserv config: %w", err)
	}

	return "", nil
}

// resolveInterfaceName checks whether baseName exists as an exact interface in
// sysfs, or whether a numbered instance (e.g., "vpns0") is available.
// Returns the resolved name or empty string if nothing matches.
func resolveInterfaceName(cfg vpnDiscoveryConfig, baseName string) string {
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return ""
	}
	// Check if exact name exists.
	for _, entry := range entries {
		if entry.Name() == baseName {
			return baseName
		}
	}
	// Scan for numbered instances (e.g., vpns0, vpns1).
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName) && len(entry.Name()) > len(baseName) {
			return entry.Name()
		}
	}
	return ""
}

// resolveAllInterfaceNames returns every entry in sysfs that matches baseName
// exactly or starts with baseName (e.g., baseName="vpns" -> "vpns0", "vpns1").
func resolveAllInterfaceNames(cfg vpnDiscoveryConfig, baseName string) []string {
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return nil
	}
	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if name == baseName || (strings.HasPrefix(name, baseName) && len(name) > len(baseName)) {
			result = append(result, name)
		}
	}
	return result
}

// DiscoverAllVPNInterfaces returns all active VPN interface names found via
// ocserv config (all numbered instances of the device directive) or sysfs scan.
// Returns nil slice when none are found, never returns an error for missing config.
func DiscoverAllVPNInterfaces(opts ...VPNDiscoveryOption) ([]string, error) {
	cfg := vpnDiscoveryConfig{
		reader:     OSReader{},
		configPath: "/etc/ocserv/ocserv.conf",
		sysNetPath: "/sys/class/net",
		readDir:    os.ReadDir,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Strategy 1: ocserv config device directive -> resolve all numbered instances.
	if baseName, err := discoverFromOcservConfig(cfg); err == nil && baseName != "" {
		if all := resolveAllInterfaceNames(cfg, baseName); len(all) > 0 {
			return all, nil
		}
	}

	// Strategy 2: sysfs scan — return all matching prefix entries.
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return nil, fmt.Errorf("read network interfaces: %w", err)
	}

	vpnPrefixes := []string{"vpns", "tun", "tap", "wg"}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		for _, prefix := range vpnPrefixes {
			if strings.HasPrefix(name, prefix) {
				found = append(found, name)
				break
			}
		}
	}
	return found, nil
}

// discoverFromSysFS scans the given directory for network interfaces that match
// typical VPN interface name prefixes. If multiple VPN interfaces are found,
// "tun0" is preferred; otherwise the first match is returned.
func discoverFromSysFS(cfg vpnDiscoveryConfig) (string, error) {
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return "", fmt.Errorf("read network interfaces: %w", err)
	}

	vpnPrefixes := []string{"vpns", "tun", "tap", "wg"}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		for _, prefix := range vpnPrefixes {
			if strings.HasPrefix(name, prefix) {
				found = append(found, name)
				break
			}
		}
	}

	if len(found) == 0 {
		return "", nil
	}

	// Prefer tun0 when multiple VPN interfaces are present.
	for _, iface := range found {
		if iface == "tun0" {
			return iface, nil
		}
	}

	return found[0], nil
}
