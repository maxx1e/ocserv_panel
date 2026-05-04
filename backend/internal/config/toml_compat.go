// Package config isolates the third-party TOML library in this file.
// Switching TOML libraries requires changing only toml_compat.go.

// toml_compat.go isolates the third-party TOML dependency.
// Switching TOML libraries requires changing only this file.

package config

import (
	"net"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

func unmarshalTOML(data []byte, cfg *Config) error {
	return toml.Unmarshal(data, cfg)
}

func unmarshalTOMLRaw(data []byte, v any) error {
	return toml.Unmarshal(data, v)
}

func marshalTOML(cfg Config) ([]byte, error) {
	return toml.Marshal(cfg)
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, DefaultPort, err
	}
	port, atoiErr := strconv.Atoi(portStr)
	if atoiErr != nil {
		return host, DefaultPort, atoiErr
	}
	return host, port, nil
}
