package declarative

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		// Loopback
		{name: "loopback 127.0.0.1", ip: "127.0.0.1", blocked: true},
		{name: "loopback 127.0.0.2", ip: "127.0.0.2", blocked: true},
		{name: "IPv6 loopback", ip: "::1", blocked: true},

		// Cloud metadata
		{name: "AWS metadata 169.254.169.254", ip: "169.254.169.254", blocked: true},
		{name: "link-local 169.254.0.1", ip: "169.254.0.1", blocked: true},

		// RFC1918 private
		{name: "10.0.0.1", ip: "10.0.0.1", blocked: true},
		{name: "10.255.255.255", ip: "10.255.255.255", blocked: true},
		{name: "172.16.0.1", ip: "172.16.0.1", blocked: true},
		{name: "172.31.255.255", ip: "172.31.255.255", blocked: true},
		{name: "192.168.1.1", ip: "192.168.1.1", blocked: true},
		{name: "192.168.0.0", ip: "192.168.0.0", blocked: true},

		// IPv6 ULA
		{name: "IPv6 ULA fd00::1", ip: "fd00::1", blocked: true},
		{name: "IPv6 ULA fc00::1", ip: "fc00::1", blocked: true},

		// IPv6 link-local
		{name: "IPv6 link-local fe80::1", ip: "fe80::1", blocked: true},

		// Carrier-grade NAT
		{name: "CGNAT 100.64.0.1", ip: "100.64.0.1", blocked: true},
		{name: "CGNAT 100.127.255.255", ip: "100.127.255.255", blocked: true},

		// Public IPs (should be allowed)
		{name: "Google DNS 8.8.8.8", ip: "8.8.8.8", blocked: false},
		{name: "Cloudflare DNS 1.1.1.1", ip: "1.1.1.1", blocked: false},
		{name: "Public 203.0.113.1", ip: "203.0.113.1", blocked: false},
		{name: "Public IPv6 2001:4860:4860::8888", ip: "2001:4860:4860::8888", blocked: false},

		// Edge cases: just outside private ranges
		{name: "172.15.255.255 (not RFC1918)", ip: "172.15.255.255", blocked: false},
		{name: "172.32.0.0 (not RFC1918)", ip: "172.32.0.0", blocked: false},
		{name: "100.63.255.255 (not CGNAT)", ip: "100.63.255.255", blocked: false},
		{name: "100.128.0.0 (not CGNAT)", ip: "100.128.0.0", blocked: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tc.ip)
			}
			got := isBlockedIP(ip)
			if got != tc.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tc.ip, got, tc.blocked)
			}
		})
	}
}

func TestParseCIDR(t *testing.T) {
	// parseCIDR should succeed for valid CIDRs
	cidr := parseCIDR("10.0.0.0/8")
	if cidr.IP == nil {
		t.Error("expected non-nil IP in parsed CIDR")
	}
}

func TestParseCIDR_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid CIDR, got none")
		}
	}()
	parseCIDR("not-a-cidr")
}

func TestNewSSRFSafeClient(t *testing.T) {
	// Verify the client is created without error
	client := NewSSRFSafeClient(30_000_000_000) // 30s
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 30_000_000_000 {
		t.Errorf("expected 30s timeout, got %v", client.Timeout)
	}
	if client.Transport == nil {
		t.Error("expected non-nil transport")
	}
}
