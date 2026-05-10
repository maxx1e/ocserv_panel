package parse

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventsJSON parses the JSON output of `occtl show events --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
// Timestamps that fail to parse are left as zero time without returning an error.
func EventsJSON(data []byte) ([]ConnectionEvent, error) {
	if len(data) == 0 {
		return []ConnectionEvent{}, nil
	}

	type rawEvent struct {
		Timestamp string `json:"timestamp"`
		Username  string `json:"username"`
		RemoteIP  string `json:"remote_ip"`
		EventType string `json:"event_type"`
		Message   string `json:"message"`
	}

	var raw []rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse events JSON: %w", err)
	}

	events := make([]ConnectionEvent, 0, len(raw))
	for _, r := range raw {
		ts := Timestamp(r.Timestamp)
		events = append(events, ConnectionEvent{
			Timestamp: ts,
			Username:  r.Username,
			RemoteIP:  r.RemoteIP,
			EventType: r.EventType,
			Message:   r.Message,
		})
	}
	return events, nil
}

// BannedIPsJSON parses the JSON output of `occtl show ip bans --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
func BannedIPsJSON(data []byte) ([]BannedIP, error) {
	if len(data) == 0 {
		return []BannedIP{}, nil
	}

	type rawBan struct {
		IP     string `json:"ip"`
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}

	var raw []rawBan
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse banned IPs JSON: %w", err)
	}

	bans := make([]BannedIP, 0, len(raw))
	for _, r := range raw {
		bans = append(bans, BannedIP{
			IP:     r.IP,
			Score:  r.Score,
			Reason: r.Reason,
		})
	}
	return bans, nil
}

// BanPointsJSON parses the JSON output of `occtl show ip ban points --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
// Timestamps that fail to parse are left as zero time without returning an error.
func BanPointsJSON(data []byte) ([]BanPointEntry, error) {
	if len(data) == 0 {
		return []BanPointEntry{}, nil
	}

	type rawBanPoint struct {
		IP     string `json:"ip"`
		Points int    `json:"points"`
		Since  string `json:"since"`
	}

	var raw []rawBanPoint
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ban points JSON: %w", err)
	}

	entries := make([]BanPointEntry, 0, len(raw))
	for _, r := range raw {
		ts := Timestamp(r.Since)
		entries = append(entries, BanPointEntry{
			IP:     r.IP,
			Points: r.Points,
			Since:  ts,
		})
	}
	return entries, nil
}

// parseTimestamp is an internal alias kept for readability.
func parseTimestamp(s string) time.Time {
	return Timestamp(s)
}
