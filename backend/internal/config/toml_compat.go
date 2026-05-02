package config

import (
	"net"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

func unmarshalTOML(data []byte, cfg *Config) error {
	return toml.Unmarshal(data, cfg)
}

func marshalTOML(cfg Config) ([]byte, error) {
	return toml.Marshal(cfg)
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 8080, err
	}
	port, atoiErr := strconv.Atoi(portStr)
	if atoiErr != nil {
		return host, 8080, atoiErr
	}
	return host, port, nil
}
