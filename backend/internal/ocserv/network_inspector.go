package ocserv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// PrerequisiteCheck describes the result of a single host-level prerequisite check.
type PrerequisiteCheck struct {
	Category   string `json:"category"`             // "kernel" | "firewall" | "routing"
	Name       string `json:"name"`
	Status     string `json:"status"`               // "configured" | "missing" | "partial" | "unknown"
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Suggestion string `json:"suggestion,omitempty"`
}

// NetworkPrerequisites aggregates all prerequisite check results.
type NetworkPrerequisites struct {
	Items []PrerequisiteCheck `json:"items"`
}

// NetworkInspector performs read-only checks of host networking prerequisites
// required for ocserv to function correctly (IP forwarding, firewall rules, etc.).
// It never modifies system state.
type NetworkInspector struct {
	runner  CommandRunner
	procDir string
}

// NewNetworkInspector returns a NetworkInspector using the real /proc/sys directory.
func NewNetworkInspector(runner CommandRunner) *NetworkInspector {
	return &NetworkInspector{runner: runner, procDir: "/proc/sys"}
}

// NewNetworkInspectorWithProcDir returns a NetworkInspector that reads kernel
// parameters from procDir instead of /proc/sys. Used in tests.
func NewNetworkInspectorWithProcDir(runner CommandRunner, procDir string) *NetworkInspector {
	return &NetworkInspector{runner: runner, procDir: procDir}
}

// Inspect runs all prerequisite checks and returns their results.
// Individual check failures are reported as "unknown" status items rather than
// returning an error, so callers always receive a complete snapshot.
func (i *NetworkInspector) Inspect(ctx context.Context) (NetworkPrerequisites, error) {
	var items []PrerequisiteCheck
	items = append(items, i.checkKernelParam(
		"net/ipv4/ip_forward",
		"IPv4 Forwarding",
		"1",
		"sysctl -w net.ipv4.ip_forward=1",
	))
	items = append(items, i.checkKernelParam(
		"net/ipv6/conf/all/forwarding",
		"IPv6 Forwarding",
		"1",
		"sysctl -w net.ipv6.conf.all.forwarding=1",
	))
	items = append(items, i.checkFirewall(ctx)...)
	return NetworkPrerequisites{Items: items}, nil
}

// checkKernelParam reads a sysctl parameter from procDir and returns a
// PrerequisiteCheck comparing the actual value to expected.
// Returns "unknown" status when the file cannot be read.
func (i *NetworkInspector) checkKernelParam(paramPath, name, expected, suggestion string) PrerequisiteCheck {
	fullPath := filepath.Join(i.procDir, paramPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return PrerequisiteCheck{
			Category:   "kernel",
			Name:       name,
			Status:     "unknown",
			Expected:   expected,
			Actual:     "",
			Suggestion: suggestion,
		}
	}

	actual := strings.TrimSpace(string(data))
	status := "missing"
	if actual == expected {
		status = "configured"
	}

	return PrerequisiteCheck{
		Category:   "kernel",
		Name:       name,
		Status:     status,
		Expected:   expected,
		Actual:     actual,
		Suggestion: suggestion,
	}
}

// checkFirewall inspects iptables rules for the presence of NAT MASQUERADE and
// vpns+ forwarding rules required by ocserv. Returns one check per rule type.
// When iptables cannot be run, all checks report "unknown".
func (i *NetworkInspector) checkFirewall(ctx context.Context) []PrerequisiteCheck {
	natResult, natErr := i.runner.Run(ctx, "iptables", []string{"-t", "nat", "-S"}, nil)
	forwardResult, fwdErr := i.runner.Run(ctx, "iptables", []string{"-S", "FORWARD"}, nil)

	masqCheck := PrerequisiteCheck{
		Category:   "firewall",
		Name:       "NAT Masquerade",
		Expected:   "MASQUERADE rule present",
		Suggestion: "Add: iptables -t nat -A POSTROUTING -o <wan-iface> -j MASQUERADE",
	}
	vpnsFwdCheck := PrerequisiteCheck{
		Category:   "firewall",
		Name:       "VPN Interface Forward",
		Expected:   "vpns+ FORWARD rule present",
		Suggestion: "Add: iptables -A FORWARD -i vpns+ -j ACCEPT",
	}

	if natErr != nil {
		masqCheck.Status = "unknown"
		masqCheck.Actual = natErr.Error()
	} else {
		if strings.Contains(natResult.Stdout, "MASQUERADE") {
			masqCheck.Status = "configured"
			masqCheck.Actual = "MASQUERADE rule found"
		} else {
			masqCheck.Status = "missing"
			masqCheck.Actual = "no MASQUERADE rule"
		}
	}

	if fwdErr != nil {
		vpnsFwdCheck.Status = "unknown"
		vpnsFwdCheck.Actual = fwdErr.Error()
	} else {
		if strings.Contains(forwardResult.Stdout, "vpns+") {
			vpnsFwdCheck.Status = "configured"
			vpnsFwdCheck.Actual = "vpns+ FORWARD rule found"
		} else {
			vpnsFwdCheck.Status = "missing"
			vpnsFwdCheck.Actual = "no vpns+ FORWARD rule"
		}
	}

	return []PrerequisiteCheck{masqCheck, vpnsFwdCheck}
}
