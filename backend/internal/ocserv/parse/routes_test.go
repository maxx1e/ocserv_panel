package parse_test

import (
	"testing"

	"ocserv_panel/backend/internal/ocserv/parse"
)

func TestRoutesTextEmpty(t *testing.T) {
	t.Parallel()
	routes, err := parse.RoutesText("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}

func TestRoutesTextHeaderOnly(t *testing.T) {
	t.Parallel()
	input := "    id     user    vhost device           vpn-ip                       iroute\n"
	routes, err := parse.RoutesText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}

func TestRoutesTextSingleRoute(t *testing.T) {
	t.Parallel()
	input := `    id     user    vhost device           vpn-ip                       iroute
 12345   testuser  default vpns0   10.10.10.100                  192.168.1.0/24`
	routes, err := parse.RoutesText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].ID != "12345" {
		t.Errorf("route[0].ID = %q, want %q", routes[0].ID, "12345")
	}
	if routes[0].Username != "testuser" {
		t.Errorf("route[0].Username = %q, want %q", routes[0].Username, "testuser")
	}
	if routes[0].Vhost != "default" {
		t.Errorf("route[0].Vhost = %q, want %q", routes[0].Vhost, "default")
	}
	if routes[0].Device != "vpns0" {
		t.Errorf("route[0].Device = %q, want %q", routes[0].Device, "vpns0")
	}
	if routes[0].VpnIP != "10.10.10.100" {
		t.Errorf("route[0].VpnIP = %q, want %q", routes[0].VpnIP, "10.10.10.100")
	}
	if routes[0].Route != "192.168.1.0/24" {
		t.Errorf("route[0].Route = %q, want %q", routes[0].Route, "192.168.1.0/24")
	}
}

func TestRoutesTextMultiple(t *testing.T) {
	t.Parallel()
	input := `    id     user    vhost device           vpn-ip                       iroute
 12345   testuser  default vpns0   10.10.10.100                  192.168.1.0/24
 12346   another   default vpns1   10.10.10.101                  172.16.0.0/16`
	routes, err := parse.RoutesText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].ID != "12345" {
		t.Errorf("route[0].ID = %q, want %q", routes[0].ID, "12345")
	}
	if routes[1].ID != "12346" {
		t.Errorf("route[1].ID = %q, want %q", routes[1].ID, "12346")
	}
	if routes[1].Route != "172.16.0.0/16" {
		t.Errorf("route[1].Route = %q, want %q", routes[1].Route, "172.16.0.0/16")
	}
}
