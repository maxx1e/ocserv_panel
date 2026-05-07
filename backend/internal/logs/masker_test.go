package logs

import (
	"strings"
	"testing"
)

func TestMaskSensitive_Password(t *testing.T) {
	input := "user set password=secret123 for admin"
	got := MaskSensitive(input)
	if got == input {
		t.Error("password not masked")
	}
	if !strings.Contains(got, "password=***") {
		t.Errorf("got %q, want password=*** pattern", got)
	}
}

func TestMaskSensitive_Token(t *testing.T) {
	input := "auth token=eyJhbGciOiJIUzI1NiJ9.abcdef123 granted"
	got := MaskSensitive(input)
	if !strings.Contains(got, "token=***") {
		t.Errorf("got %q, want token=*** pattern", got)
	}
}

func TestMaskSensitive_Secret(t *testing.T) {
	input := "secret=mysecretvalue123 loaded"
	got := MaskSensitive(input)
	if !strings.Contains(got, "secret=***") {
		t.Errorf("got %q, want secret=*** pattern", got)
	}
}

func TestMaskSensitive_Authorization(t *testing.T) {
	input := "Authorization: Bearer eyJhbG123456"
	got := MaskSensitive(input)
	if !strings.Contains(got, "Authorization: ***") {
		t.Errorf("got %q, want Authorization: *** pattern", got)
	}
}

func TestMaskSensitive_Cookie(t *testing.T) {
	input := "cookie=session_abc123def456 path=/"
	got := MaskSensitive(input)
	if !strings.Contains(got, "cookie=***") {
		t.Errorf("got %q, want cookie=*** pattern", got)
	}
}

func TestMaskSensitive_NoFalsePositive(t *testing.T) {
	input := "user admin connected from 192.168.1.1 with TLS 1.3"
	got := MaskSensitive(input)
	if got != input {
		t.Errorf("false positive: got %q, want %q", got, input)
	}
}

func TestMaskSensitive_MultiplePatterns(t *testing.T) {
	input := "password=abc token=xyz123456789 done"
	got := MaskSensitive(input)
	if strings.Contains(got, "abc") || strings.Contains(got, "xyz123456789") {
		t.Errorf("not all patterns masked: %q", got)
	}
}
