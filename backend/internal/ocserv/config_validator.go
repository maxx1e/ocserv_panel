package ocserv

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigIssueSeverity rates the importance of a configuration issue.
type ConfigIssueSeverity string

const (
	IssueError   ConfigIssueSeverity = "error"
	IssueWarning ConfigIssueSeverity = "warning"
	IssueInfo    ConfigIssueSeverity = "info"
)

// ConfigIssue describes a single ocserv configuration problem.
type ConfigIssue struct {
	Severity   ConfigIssueSeverity `json:"severity"`
	Key        string              `json:"key"`
	Message    string              `json:"message"`
	Suggestion string              `json:"suggestion,omitempty"`
}

// ConfigValidator checks ocserv configuration for common production issues.
// It performs static analysis of the config file — it does not run ocserv.
type ConfigValidator struct {
	configPath string
}

// NewConfigValidator returns a validator for the given ocserv config path.
func NewConfigValidator(configPath string) *ConfigValidator {
	return &ConfigValidator{configPath: configPath}
}

// ValidateConfig reads and checks the ocserv configuration file for issues.
// Returns a slice of ConfigIssue (may be empty). Only returns an error
// when the file cannot be read for reasons other than not existing.
func (v *ConfigValidator) ValidateConfig(ctx context.Context) ([]ConfigIssue, error) {
	_ = ctx

	data, err := os.ReadFile(v.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConfigIssue{{
				Severity: IssueError,
				Key:      "config_file",
				Message:  fmt.Sprintf("ocserv config file not found: %s", v.configPath),
				Suggestion: fmt.Sprintf(
					"Create the config file at %s or update the installation.ocserv_config_path setting.",
					v.configPath),
			}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", v.configPath, err)
	}

	var issues []ConfigIssue
	configDir := filepath.Dir(v.configPath)

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		key, value, ok := parseConfigLine(line)
		if !ok {
			continue
		}

		switch key {
		case "server-cert", "server-key", "ca-cert":
			issues = append(issues, v.checkCertPath(key, value, configDir)...)
		case "auth":
			issues = append(issues, v.checkAuthDirective(key, value, configDir)...)
		case "socket-file":
			issues = append(issues, v.checkSocketPath(key, value)...)
		case "device":
			issues = append(issues, v.checkDevice(key, value)...)
		}
	}

	return issues, nil
}

// ValidateContent validates ocserv config text without reading from disk.
// It checks for syntax errors and common misconfiguration in the supplied content.
// Returns a slice of ConfigIssue (may be empty). Never returns a non-nil error.
func (v *ConfigValidator) ValidateContent(ctx context.Context, content string) ([]ConfigIssue, error) {
	_ = ctx

	var issues []ConfigIssue
	for i, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseConfigLine(line)
		if !ok {
			issues = append(issues, ConfigIssue{
				Severity:   IssueError,
				Key:        fmt.Sprintf("line_%d", i+1),
				Message:    fmt.Sprintf("malformed config line (missing '='): %s", line),
				Suggestion: "Each setting must follow the format: key = value",
			})
			continue
		}

		switch key {
		case "tcp-port", "udp-port":
			issues = append(issues, v.checkPortValue(key, value)...)
		case "route", "no-route":
			issues = append(issues, v.checkCIDR(key, value)...)
		case "dns":
			issues = append(issues, v.checkDNS(key, value)...)
		}
	}

	return issues, nil
}

// checkPortValue validates that value is an integer in the valid port range [1, 65535].
func (v *ConfigValidator) checkPortValue(key, value string) []ConfigIssue {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return []ConfigIssue{{
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s has invalid port value: %s (must be 1-65535)", key, value),
			Suggestion: fmt.Sprintf("Set %s to an integer between 1 and 65535.", key),
		}}
	}
	return nil
}

// checkCIDR validates that value is a valid CIDR notation address block.
func (v *ConfigValidator) checkCIDR(key, value string) []ConfigIssue {
	_, _, err := net.ParseCIDR(value)
	if err != nil {
		return []ConfigIssue{{
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s has invalid CIDR value: %s", key, value),
			Suggestion: fmt.Sprintf("Use valid CIDR notation, for example: 10.0.0.0/8"),
		}}
	}
	return nil
}

// checkDNS validates that value is a valid IP address for a DNS server.
func (v *ConfigValidator) checkDNS(key, value string) []ConfigIssue {
	if net.ParseIP(value) == nil {
		return []ConfigIssue{{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("%s has invalid IP address: %s", key, value),
			Suggestion: "Set dns to a valid IPv4 or IPv6 address, for example: 8.8.8.8",
		}}
	}
	return nil
}

func (v *ConfigValidator) checkCertPath(key, value, configDir string) []ConfigIssue {
	var issues []ConfigIssue

	// Relative paths are a warning because they can break when ocserv is
	// started from a different working directory.
	if !filepath.IsAbs(value) {
		issues = append(issues, ConfigIssue{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("%s uses a relative path: %s", key, value),
			Suggestion: fmt.Sprintf("Use an absolute path like /etc/ocserv/certs/%s", filepath.Base(value)),
		})
	}

	// Resolve to absolute for existence check.
	fullPath := value
	if !filepath.IsAbs(value) {
		fullPath = filepath.Join(configDir, value)
	}

	if _, err := os.Stat(fullPath); err != nil {
		issues = append(issues, ConfigIssue{
			Severity: IssueError,
			Key:      key,
			Message:  fmt.Sprintf("%s file not found: %s (resolved: %s)", key, value, fullPath),
			Suggestion: fmt.Sprintf(
				"Create the certificate file at %s or update the %s directive.", fullPath, key),
		})
	}

	return issues
}

func (v *ConfigValidator) checkAuthDirective(key, value, configDir string) []ConfigIssue {
	var issues []ConfigIssue

	if !strings.HasPrefix(value, "plain[passwd=") {
		return issues
	}

	passwdFile := strings.TrimPrefix(value, "plain[passwd=")
	passwdFile = strings.TrimSuffix(passwdFile, "]")

	if !filepath.IsAbs(passwdFile) {
		issues = append(issues, ConfigIssue{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("auth password file uses a relative path: %s", passwdFile),
			Suggestion: "Use an absolute path like /etc/ocserv/ocpasswd",
		})
	}

	fullPath := passwdFile
	if !filepath.IsAbs(passwdFile) {
		fullPath = filepath.Join(configDir, passwdFile)
	}

	if _, err := os.Stat(fullPath); err != nil {
		issues = append(issues, ConfigIssue{
			Severity:   IssueInfo,
			Key:        key,
			Message:    fmt.Sprintf("auth password file not found: %s (resolved: %s)", passwdFile, fullPath),
			Suggestion: "The password file will be created when the first user is added.",
		})
	}

	return issues
}

func (v *ConfigValidator) checkSocketPath(key, value string) []ConfigIssue {
	var issues []ConfigIssue
	if !filepath.IsAbs(value) {
		issues = append(issues, ConfigIssue{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("%s uses a relative path: %s", key, value),
			Suggestion: "Use an absolute path like /var/run/ocserv-socket",
		})
	}
	return issues
}

func (v *ConfigValidator) checkDevice(key, value string) []ConfigIssue {
	var issues []ConfigIssue
	// The TUN device only exists while ocserv is running, so this is info-level.
	if _, err := os.Stat("/sys/class/net/" + value); err != nil {
		issues = append(issues, ConfigIssue{
			Severity:   IssueInfo,
			Key:        key,
			Message:    fmt.Sprintf("network device %s not found (expected when ocserv is not running)", value),
			Suggestion: fmt.Sprintf("The device %s will appear when ocserv starts.", value),
		})
	}
	return issues
}

// parseConfigLine parses a single "key = value" line from an ocserv config file.
// Returns the key, value, and true if parsing succeeded.
// Surrounding whitespace and quotes are removed from the value.
func parseConfigLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	// Strip surrounding single or double quotes.
	value = strings.Trim(value, `"'`)
	return key, value, true
}
