package ocserv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
