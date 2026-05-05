package ocserv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkInspectorIPv4ForwardEnabled(t *testing.T) {
	t.Parallel()
	procDir := t.TempDir()
	paramDir := filepath.Join(procDir, "net", "ipv4")
	if err := os.MkdirAll(paramDir, 0755); err != nil {
		t.Fatalf("create proc dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paramDir, "ip_forward"), []byte("1\n"), 0644); err != nil {
		t.Fatalf("write ip_forward: %v", err)
	}

	runner := NewFakeCommandRunner()
	inspector := NewNetworkInspectorWithProcDir(runner, procDir)
	result, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	for _, item := range result.Items {
		if item.Name == "IPv4 Forwarding" {
			if item.Status != "configured" {
				t.Errorf("IPv4 Forwarding status = %q, want %q", item.Status, "configured")
			}
			return
		}
	}
	t.Fatal("IPv4 Forwarding check not found in results")
}

func TestNetworkInspectorIPv6ForwardMissing(t *testing.T) {
	t.Parallel()
	procDir := t.TempDir()
	paramDir := filepath.Join(procDir, "net", "ipv6", "conf", "all")
	if err := os.MkdirAll(paramDir, 0755); err != nil {
		t.Fatalf("create proc dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paramDir, "forwarding"), []byte("0\n"), 0644); err != nil {
		t.Fatalf("write forwarding: %v", err)
	}

	runner := NewFakeCommandRunner()
	inspector := NewNetworkInspectorWithProcDir(runner, procDir)
	result, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	for _, item := range result.Items {
		if item.Name == "IPv6 Forwarding" {
			if item.Status != "missing" {
				t.Errorf("IPv6 Forwarding status = %q, want %q", item.Status, "missing")
			}
			return
		}
	}
	t.Fatal("IPv6 Forwarding check not found in results")
}

func TestNetworkInspectorProcNotReadable(t *testing.T) {
	t.Parallel()
	// Use a nonexistent proc dir — all kernel param reads fail.
	runner := NewFakeCommandRunner()
	inspector := NewNetworkInspectorWithProcDir(runner, "/nonexistent/proc/sys")
	result, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	kernelChecks := 0
	for _, item := range result.Items {
		if item.Category == "kernel" {
			kernelChecks++
			if item.Status != "unknown" {
				t.Errorf("kernel check %q status = %q, want %q", item.Name, item.Status, "unknown")
			}
		}
	}
	if kernelChecks == 0 {
		t.Fatal("expected at least one kernel check, got none")
	}
}

func TestNetworkInspectorFirewallIptables(t *testing.T) {
	t.Parallel()
	procDir := t.TempDir()
	runner := NewFakeCommandRunner()
	// Simulate iptables -t nat -S output containing a MASQUERADE rule.
	runner.Results["iptables -t nat -S"] = CommandResult{
		Stdout:   "-A POSTROUTING -o eth0 -j MASQUERADE\n",
		ExitCode: 0,
	}
	// Simulate iptables -S FORWARD output containing vpns+.
	runner.Results["iptables -S FORWARD"] = CommandResult{
		Stdout:   "-A FORWARD -i vpns+ -j ACCEPT\n",
		ExitCode: 0,
	}

	inspector := NewNetworkInspectorWithProcDir(runner, procDir)
	result, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	masqFound := false
	vpnsFound := false
	for _, item := range result.Items {
		switch item.Name {
		case "NAT Masquerade":
			masqFound = true
			if item.Status != "configured" {
				t.Errorf("NAT Masquerade status = %q, want configured", item.Status)
			}
		case "VPN Interface Forward":
			vpnsFound = true
			if item.Status != "configured" {
				t.Errorf("VPN Interface Forward status = %q, want configured", item.Status)
			}
		}
	}
	if !masqFound {
		t.Error("NAT Masquerade check not found in results")
	}
	if !vpnsFound {
		t.Error("VPN Interface Forward check not found in results")
	}
}

func TestNetworkInspector_ListenerPortConfigured(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("tcp-port = 443\nudp-port = 443\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "-A POSTROUTING -j MASQUERADE\n", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "-A FORWARD -i vpns+ -j ACCEPT\n", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "-A INPUT -p tcp --dport 443 -j ACCEPT\n", ExitCode: 0}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var portCheck *PrerequisiteCheck
	for i := range result.Items {
		if result.Items[i].Name == "TCP Port 443 Allowed" {
			portCheck = &result.Items[i]
			break
		}
	}
	require.NotNil(t, portCheck, "expected TCP port check")
	assert.Equal(t, "configured", portCheck.Status)
}

func TestNetworkInspector_SubnetOverlapDetected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("ipv4-network = 10.10.10.0/24\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "-A POSTROUTING -j MASQUERADE\n", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "-A FORWARD -i vpns+ -j ACCEPT\n", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["ip route show"] = CommandResult{
		Stdout:   "10.0.0.0/8 dev eth0 proto kernel scope link src 10.0.0.1\n192.168.1.0/24 dev wlan0\n",
		ExitCode: 0,
	}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var overlapCheck *PrerequisiteCheck
	for i := range result.Items {
		if result.Items[i].Name == "VPN Subnet Overlap" {
			overlapCheck = &result.Items[i]
			break
		}
	}
	require.NotNil(t, overlapCheck, "expected VPN Subnet Overlap check")
	assert.Equal(t, "missing", overlapCheck.Status)
	assert.Contains(t, overlapCheck.Actual, "10.0.0.0/8")
}

func TestNetworkInspector_SubnetNoOverlap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("ipv4-network = 172.16.100.0/24\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["ip route show"] = CommandResult{
		Stdout:   "10.0.0.0/8 dev eth0\n192.168.1.0/24 dev wlan0\n",
		ExitCode: 0,
	}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var overlapCheck *PrerequisiteCheck
	for i := range result.Items {
		if result.Items[i].Name == "VPN Subnet Overlap" {
			overlapCheck = &result.Items[i]
			break
		}
	}
	require.NotNil(t, overlapCheck, "expected VPN Subnet Overlap check")
	assert.Equal(t, "configured", overlapCheck.Status)
	assert.Contains(t, overlapCheck.Actual, "no overlap")
}

func TestNetworkInspector_ListenerPortMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("tcp-port = 8443\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "-A INPUT -p tcp --dport 443 -j ACCEPT\n", ExitCode: 0}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var portCheck *PrerequisiteCheck
	for i := range result.Items {
		if result.Items[i].Name == "TCP Port 8443 Allowed" {
			portCheck = &result.Items[i]
			break
		}
	}
	require.NotNil(t, portCheck, "expected TCP port 8443 check")
	assert.Equal(t, "missing", portCheck.Status)
}

func TestNetworkInspector_DNSCheckValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("dns = 8.8.8.8\ndns = 1.1.1.1\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["ip route show"] = CommandResult{Stdout: "", ExitCode: 0}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var dnsChecks []PrerequisiteCheck
	for _, item := range result.Items {
		if strings.HasPrefix(item.Name, "DNS Server") {
			dnsChecks = append(dnsChecks, item)
		}
	}
	require.Len(t, dnsChecks, 2)
	assert.Equal(t, "configured", dnsChecks[0].Status)
	assert.Equal(t, "configured", dnsChecks[1].Status)
}

func TestNetworkInspector_DNSCheckInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("dns = 8.8.8.8\ndns = notanip\n"), 0644))

	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv4"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv4/ip_forward"), []byte("1\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "net/ipv6/conf/all"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "net/ipv6/conf/all/forwarding"), []byte("1\n"), 0644))

	runner := NewFakeCommandRunner()
	runner.Results["iptables -t nat -S"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S FORWARD"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["iptables -S INPUT"] = CommandResult{Stdout: "", ExitCode: 0}
	runner.Results["ip route show"] = CommandResult{Stdout: "", ExitCode: 0}

	insp := NewNetworkInspectorWithConfig(runner, procDir, cfgPath)
	result, err := insp.Inspect(context.Background())
	require.NoError(t, err)

	var dnsChecks []PrerequisiteCheck
	for _, item := range result.Items {
		if strings.HasPrefix(item.Name, "DNS Server") {
			dnsChecks = append(dnsChecks, item)
		}
	}
	require.Len(t, dnsChecks, 2)
	assert.Equal(t, "configured", dnsChecks[0].Status)
	assert.Equal(t, "missing", dnsChecks[1].Status)
	assert.Contains(t, dnsChecks[1].Actual, "invalid")
}
