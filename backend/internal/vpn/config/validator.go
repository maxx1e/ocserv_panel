package config

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
	Line       int                 `json:"line,omitempty"`
	Severity   ConfigIssueSeverity `json:"severity"`
	Key        string              `json:"key"`
	Message    string              `json:"message"`
	Suggestion string              `json:"suggestion,omitempty"`
	FixedValue string              `json:"fixedValue,omitempty"`
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

	for i, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		key, value, ok := ParseConfigLine(line)
		if !ok {
			continue
		}

		switch key {
		case "server-cert", "server-key", "ca-cert":
			for _, issue := range v.checkCertPath(key, value, configDir) {
				issue.Line = i + 1
				issues = append(issues, issue)
			}
		case "auth":
			for _, issue := range v.checkAuthDirective(key, value, configDir) {
				issue.Line = i + 1
				issues = append(issues, issue)
			}
		case "socket-file":
			for _, issue := range v.checkSocketPath(key, value) {
				issue.Line = i + 1
				issues = append(issues, issue)
			}
		case "device":
			for _, issue := range v.checkDevice(key, value) {
				issue.Line = i + 1
				issues = append(issues, issue)
			}
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

		key, value, ok := ParseConfigLine(line)
		if !ok {
			issues = append(issues, ConfigIssue{
				Line:       i + 1,
				Severity:   IssueError,
				Key:        fmt.Sprintf("line_%d", i+1),
				Message:    fmt.Sprintf("malformed config line (missing '='): %s", line),
				Suggestion: "Each setting must follow the format: key = value",
			})
			continue
		}

		// Check for unclosed quotes in the raw value portion of the line.
		rawValue := line[strings.Index(line, "=")+1:]
		if char, unclosed := hasUnclosedQuotes(rawValue); unclosed {
			issues = append(issues, ConfigIssue{
				Line:       i + 1,
				Severity:   IssueError,
				Key:        key,
				Message:    fmt.Sprintf("%s has unclosed %c quote", key, char),
				Suggestion: fmt.Sprintf("Ensure all %c quotes in the value are properly closed.", char),
			})
			continue
		}

		switch key {
		case "tcp-port", "udp-port":
			issues = append(issues, v.checkPortValue(key, value, i+1)...)
		case "route", "no-route":
			issues = append(issues, v.checkCIDR(key, value, i+1, line)...)
		case "dns":
			issues = append(issues, v.checkDNS(key, value, i+1)...)
		}
	}

	return issues, nil
}

// checkPortValue validates that value is an integer in the valid port range [1, 65535].
func (v *ConfigValidator) checkPortValue(key, value string, lineNum int) []ConfigIssue {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return []ConfigIssue{{
			Line:       lineNum,
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s has invalid port value: %s (must be 1-65535)", key, value),
			Suggestion: fmt.Sprintf("Set %s to an integer between 1 and 65535.", key),
		}}
	}
	return nil
}

// checkCIDR validates that value is a valid CIDR notation address block.
// It detects netmask notation (e.g., 10.0.0.0/255.255.255.0) and suggests CIDR replacement.
// rawLine is the full trimmed config line, used to construct FixedValue for auto-fix.
func (v *ConfigValidator) checkCIDR(key, value string, lineNum int, rawLine string) []ConfigIssue {
	_ = rawLine // used for documentation; FixedValue is constructed from key + cidr

	// Check for netmask notation first (e.g., 10.0.0.0/255.255.255.0)
	if ip, mask, ok := isNetmaskNotation(value); ok {
		cidr, valid := netmaskToCIDR(ip, mask)
		if valid {
			return []ConfigIssue{{
				Line:       lineNum,
				Severity:   IssueWarning,
				Key:        key,
				Message:    fmt.Sprintf("%s uses netmask notation: %s", key, value),
				Suggestion: fmt.Sprintf("Use CIDR notation: %s", cidr),
				FixedValue: fmt.Sprintf("%s = %s", key, cidr),
			}}
		}
		// Non-contiguous mask — error, no fix available
		return []ConfigIssue{{
			Line:       lineNum,
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s has non-contiguous subnet mask: %s", key, value),
			Suggestion: "Use a valid contiguous mask or CIDR prefix notation.",
		}}
	}

	// Standard CIDR validation
	_, _, err := net.ParseCIDR(value)
	if err != nil {
		return []ConfigIssue{{
			Line:       lineNum,
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s has invalid CIDR value: %s", key, value),
			Suggestion: "Use valid CIDR notation, for example: 10.0.0.0/8",
		}}
	}
	return nil
}

// checkDNS validates that value is a valid IP address for a DNS server.
func (v *ConfigValidator) checkDNS(key, value string, lineNum int) []ConfigIssue {
	if net.ParseIP(value) == nil {
		return []ConfigIssue{{
			Line:       lineNum,
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
			Suggestion: "Use an absolute path like /var/run/occtl.socket",
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

// ParseConfigLine parses a single "key = value" line from an ocserv config file.
// Returns the key, value, and true if parsing succeeded.
// Surrounding whitespace and quotes are removed from the value.
func ParseConfigLine(line string) (key, value string, ok bool) {
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

// hasUnclosedQuotes checks if a raw value string has an odd number of
// double or single quotes, indicating an unclosed quote.
func hasUnclosedQuotes(rawValue string) (char byte, unclosed bool) {
	dq := strings.Count(rawValue, "\"")
	if dq%2 != 0 {
		return '"', true
	}
	sq := strings.Count(rawValue, "'")
	if sq%2 != 0 {
		return '\'', true
	}
	return 0, false
}

// isNetmaskNotation checks if a value is in "ip/dotted-quad-mask" format.
// Returns the ip and mask parts if detected. Returns false for prefix notation like "10.0.0.0/24".
func isNetmaskNotation(value string) (ip, mask string, ok bool) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	ip = parts[0]
	mask = parts[1]
	if strings.Count(mask, ".") != 3 {
		return "", "", false
	}
	if net.ParseIP(ip) == nil || net.ParseIP(mask) == nil {
		return "", "", false
	}
	return ip, mask, true
}

// netmaskToCIDR converts an IP and dotted-quad mask to CIDR notation.
// Returns ("", false) if the mask is non-contiguous (invalid).
func netmaskToCIDR(ip, mask string) (string, bool) {
	maskIP := net.ParseIP(mask)
	if maskIP == nil {
		return "", false
	}
	maskBytes := maskIP.To4()
	if maskBytes == nil {
		return "", false
	}
	ipMask := net.IPMask(maskBytes)
	ones, bits := ipMask.Size()
	if bits == 0 {
		return "", false
	}
	return ip + "/" + strconv.Itoa(ones), true
}
