package ocserv

import (
	"fmt"
	"strings"
)

// restartRequiredDirectives are ocserv directives that require a full service
// restart (not just reload) when their value changes.
var restartRequiredDirectives = map[string]bool{
	"tcp-port":    true,
	"udp-port":    true,
	"server-cert": true,
	"server-key":  true,
	"listen-host": true,
	"device":      true,
}

// RestartAnalysis contains the result of comparing two config versions for
// restart-requiring changes.
type RestartAnalysis struct {
	RequiresRestart bool     `json:"requiresRestart"`
	Reasons         []string `json:"reasons,omitempty"`
}

// RestartDetector compares two config versions and determines whether a full
// service restart is needed (vs a simple reload).
type RestartDetector struct{}

// NewRestartDetector creates a new RestartDetector.
func NewRestartDetector() *RestartDetector {
	return &RestartDetector{}
}

// Detect compares oldContent and newContent, returning an analysis of whether
// any restart-required directives changed.
func (d *RestartDetector) Detect(oldContent, newContent string) RestartAnalysis {
	oldValues := parseDirectiveValues(oldContent, restartRequiredDirectives)
	newValues := parseDirectiveValues(newContent, restartRequiredDirectives)

	var reasons []string
	for key := range restartRequiredDirectives {
		oldVal := oldValues[key]
		newVal := newValues[key]
		if oldVal != newVal {
			if oldVal == "" {
				reasons = append(reasons, fmt.Sprintf("%s added: %s", key, newVal))
			} else if newVal == "" {
				reasons = append(reasons, fmt.Sprintf("%s removed (was: %s)", key, oldVal))
			} else {
				reasons = append(reasons, fmt.Sprintf("%s changed: %s → %s", key, oldVal, newVal))
			}
		}
	}

	return RestartAnalysis{
		RequiresRestart: len(reasons) > 0,
		Reasons:         reasons,
	}
}

// parseDirectiveValues extracts the last value for each directive in the filter set.
func parseDirectiveValues(content string, filter map[string]bool) map[string]string {
	values := make(map[string]string)
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := parseConfigLine(line)
		if !ok {
			continue
		}
		if filter[key] {
			values[key] = value
		}
	}
	return values
}
