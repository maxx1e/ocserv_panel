package sessions_test

import (
	"testing"

	"ocserv_panel/backend/internal/vpn/sessions"

	"github.com/stretchr/testify/assert"
)

func TestIsNetmaskNotation(t *testing.T) {
	tests := []struct {
		input    string
		wantIP   string
		wantMask string
		wantOK   bool
	}{
		{"10.10.10.0/255.255.255.0", "10.10.10.0", "255.255.255.0", true},
		{"192.168.0.0/255.255.0.0", "192.168.0.0", "255.255.0.0", true},
		{"10.0.0.0/255.0.0.0", "10.0.0.0", "255.0.0.0", true},
		{"10.0.0.0/24", "", "", false},        // prefix notation, not netmask
		{"10.0.0.0/8", "", "", false},         // prefix notation
		{"invalid", "", "", false},            // no slash
		{"10.0.0.0/abc", "", "", false},       // not a dotted quad
		{"10.0.0.0/256.0.0.0", "", "", false}, // invalid IP
		{"", "", "", false},                   // empty
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ip, mask, ok := sessions.IsNetmaskNotation(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantIP, ip)
				assert.Equal(t, tt.wantMask, mask)
			}
		})
	}
}

func TestNetmaskToCIDR(t *testing.T) {
	tests := []struct {
		ip     string
		mask   string
		want   string
		wantOK bool
	}{
		{"10.10.10.0", "255.255.255.0", "10.10.10.0/24", true},
		{"192.168.0.0", "255.255.0.0", "192.168.0.0/16", true},
		{"10.0.0.0", "255.0.0.0", "10.0.0.0/8", true},
		{"10.0.0.0", "255.255.128.0", "10.0.0.0/17", true},
		{"172.16.0.0", "255.240.0.0", "172.16.0.0/12", true},
		{"192.168.5.0", "255.255.255.0", "192.168.5.0/24", true},
		{"10.0.0.0", "255.255.100.0", "", false}, // non-contiguous
		{"10.0.0.0", "0.255.255.0", "", false},   // non-contiguous
		{"10.0.0.0", "invalid", "", false},        // not an IP
	}
	for _, tt := range tests {
		t.Run(tt.ip+"/"+tt.mask, func(t *testing.T) {
			result, ok := sessions.NetmaskToCIDR(tt.ip, tt.mask)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}
