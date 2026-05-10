package parse

import (
	"strings"
)

// RoutesText parses the text output of `occtl show iroutes`.
func RoutesText(text string) ([]UserRoute, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []UserRoute{}, nil
	}

	lines := strings.Split(text, "\n")
	var routes []UserRoute

	headerSkipped := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip header line
		if !headerSkipped {
			if strings.Contains(trimmed, "iroute") || strings.Contains(trimmed, "id") {
				headerSkipped = true
				continue
			}
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue // incomplete line, skip
		}

		route := UserRoute{
			ID:       fields[0],
			Username: fields[1],
			Vhost:    fields[2],
			Device:   fields[3],
			VpnIP:    fields[4],
			Route:    strings.Join(fields[5:], " "),
		}
		routes = append(routes, route)
	}

	if routes == nil {
		routes = []UserRoute{}
	}
	return routes, nil
}
