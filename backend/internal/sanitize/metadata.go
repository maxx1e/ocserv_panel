// Package sanitize provides shared sanitization logic for sensitive metadata.
package sanitize

import "strings"

// Metadata returns a copy of metadata with sensitive keys and values replaced
// by "[redacted]". A nil input returns an empty non-nil map.
func Metadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return map[string]string{}
	}
	sanitized := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if isSensitiveKey(key) || isSensitiveValue(value) {
			sanitized[key] = "[redacted]"
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

func isSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	for _, sensitive := range []string{
		"password",
		"passphrase",
		"token",
		"sessiontoken",
		"secret",
		"privatekey",
		"credential",
		"authorization",
		"cookie",
	} {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}

func isSensitiveValue(value string) bool {
	upper := strings.ToUpper(value)
	return strings.Contains(upper, "BEGIN") && strings.Contains(upper, "PRIVATE KEY")
}

func normalizeKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(key))
}
