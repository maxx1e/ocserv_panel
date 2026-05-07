package logs

import "regexp"

// sensitivePatterns defines regex patterns and their replacements for masking.
var sensitivePatterns = []struct {
	re          *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)password[=:]\s*\S+`), "password=***"},
	{regexp.MustCompile(`(?i)token[=:]\s*[A-Za-z0-9+/=_.\-]{8,}`), "token=***"},
	{regexp.MustCompile(`(?i)(key|secret|credential)[=:]\s*\S+`), "${1}=***"},
	{regexp.MustCompile(`(?i)cookie[=:]\s*\S+`), "cookie=***"},
	{regexp.MustCompile(`(?i)Authorization:\s*\S+`), "Authorization: ***"},
}

// MaskSensitive replaces sensitive values in a log line with redacted placeholders.
func MaskSensitive(s string) string {
	for _, p := range sensitivePatterns {
		s = p.re.ReplaceAllString(s, p.replacement)
	}
	return s
}

// MaskLogLine applies masking to both the Message and Raw fields of a LogLine.
func MaskLogLine(line *LogLine) {
	line.Message = MaskSensitive(line.Message)
	line.Raw = MaskSensitive(line.Raw)
}
