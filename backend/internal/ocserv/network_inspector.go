package ocserv

import (
	"context"
	"fmt"
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
	runner     CommandRunner
	procDir    string
	configPath string
}

// NewNetworkInspector returns a NetworkInspector using the real /proc/sys directory.
func NewNetworkInspector(runner CommandRunner) *NetworkInspector {
	return &NetworkInspector{runner: runner, procDir: "/proc/sys", configPath: ""}
}

// NewNetworkInspectorWithProcDir returns a NetworkInspector that reads kernel
// parameters from procDir instead of /proc/sys. Used in tests.
func NewNetworkInspectorWithProcDir(runner CommandRunner, procDir string) *NetworkInspector {
	return &NetworkInspector{runner: runner, procDir: procDir, configPath: ""}
}

// NewNetworkInspectorWithConfig returns a NetworkInspector that reads kernel
// parameters from procDir and parses the ocserv config at configPath to check
// whether listener ports are allowed through the firewall.
func NewNetworkInspectorWithConfig(runner CommandRunner, procDir, configPath string) *NetworkInspector {
	return &NetworkInspector{runner: runner, procDir: procDir, configPath: configPath}
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
	items = append(items, i.checkListenerPorts(ctx)...)
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

// checkListenerPorts reads the ocserv config to determine the tcp-port and
// udp-port values, then checks whether the host firewall has INPUT ACCEPT rules
// for those ports. When configPath is empty, the check is skipped gracefully.
func (i *NetworkInspector) checkListenerPorts(ctx context.Context) []PrerequisiteCheck {
	if i.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(i.configPath)
	if err != nil {
		return nil
	}

	var tcpPort, udpPort string
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := parseConfigLine(line)
		if !ok {
			continue
		}
		switch key {
		case "tcp-port":
			tcpPort = value
		case "udp-port":
			udpPort = value
		}
	}

	if tcpPort == "" && udpPort == "" {
		return nil
	}

	inputResult, err := i.runner.Run(ctx, "iptables", []string{"-S", "INPUT"}, nil)
	if err != nil {
		var checks []PrerequisiteCheck
		if tcpPort != "" {
			checks = append(checks, PrerequisiteCheck{
				Category: "firewall", Name: fmt.Sprintf("TCP Port %s Allowed", tcpPort),
				Status: "unknown", Expected: "ACCEPT rule for TCP dport " + tcpPort,
				Actual: err.Error(), Suggestion: fmt.Sprintf("iptables -A INPUT -p tcp --dport %s -j ACCEPT", tcpPort),
			})
		}
		if udpPort != "" && udpPort != tcpPort {
			checks = append(checks, PrerequisiteCheck{
				Category: "firewall", Name: fmt.Sprintf("UDP Port %s Allowed", udpPort),
				Status: "unknown", Expected: "ACCEPT rule for UDP dport " + udpPort,
				Actual: err.Error(), Suggestion: fmt.Sprintf("iptables -A INPUT -p udp --dport %s -j ACCEPT", udpPort),
			})
		}
		return checks
	}

	var checks []PrerequisiteCheck
	output := inputResult.Stdout

	if tcpPort != "" {
		check := PrerequisiteCheck{
			Category: "firewall", Name: fmt.Sprintf("TCP Port %s Allowed", tcpPort),
			Expected: "ACCEPT rule for TCP dport " + tcpPort,
			Suggestion: fmt.Sprintf("iptables -A INPUT -p tcp --dport %s -j ACCEPT", tcpPort),
		}
		if strings.Contains(output, "--dport "+tcpPort) || strings.Contains(output, "-p tcp -j ACCEPT") {
			check.Status = "configured"
			check.Actual = "matching rule found"
		} else {
			check.Status = "missing"
			check.Actual = "no matching INPUT rule"
		}
		checks = append(checks, check)
	}
	if udpPort != "" && udpPort != tcpPort {
		check := PrerequisiteCheck{
			Category: "firewall", Name: fmt.Sprintf("UDP Port %s Allowed", udpPort),
			Expected: "ACCEPT rule for UDP dport " + udpPort,
			Suggestion: fmt.Sprintf("iptables -A INPUT -p udp --dport %s -j ACCEPT", udpPort),
		}
		if strings.Contains(output, "--dport "+udpPort) || strings.Contains(output, "-p udp -j ACCEPT") {
			check.Status = "configured"
			check.Actual = "matching rule found"
		} else {
			check.Status = "missing"
			check.Actual = "no matching INPUT rule"
		}
		checks = append(checks, check)
	}
	return checks
}
