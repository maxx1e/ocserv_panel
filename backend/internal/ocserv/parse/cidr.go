package parse

import (
	"net"
	"strconv"
	"strings"
)

// IsNetmaskNotation checks if a value is in "ip/dotted-quad-mask" format (e.g., "10.0.0.0/255.255.255.0").
// Returns the ip and mask parts if detected. Returns false for prefix notation like "10.0.0.0/24".
func IsNetmaskNotation(value string) (ip, mask string, ok bool) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	ip = parts[0]
	mask = parts[1]
	// A dotted-quad mask has exactly 3 dots. A prefix integer has none.
	if strings.Count(mask, ".") != 3 {
		return "", "", false
	}
	// Both parts must parse as valid IPs
	if net.ParseIP(ip) == nil || net.ParseIP(mask) == nil {
		return "", "", false
	}
	return ip, mask, true
}

// NetmaskToCIDR converts an IP and dotted-quad mask to CIDR notation.
// Returns ("", false) if the mask is non-contiguous (invalid).
func NetmaskToCIDR(ip, mask string) (string, bool) {
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
	// Size() returns (0, 0) for non-contiguous masks
	if bits == 0 {
		return "", false
	}
	return ip + "/" + strconv.Itoa(ones), true
}
