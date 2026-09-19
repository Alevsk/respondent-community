package declarative

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// blockedCIDRs contains IP ranges that must not be accessed by declarative sources.
// This blocks SSRF to internal services, cloud metadata endpoints, and localhost.
var blockedCIDRs []net.IPNet

func init() {
	cidrs := []string{
		// Loopback
		"127.0.0.0/8",
		"::1/128",
		// RFC1918 private
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		// Link-local (includes AWS/GCP/Azure metadata at 169.254.169.254)
		"169.254.0.0/16",
		"fe80::/10",
		// IPv6 ULA (includes AWS IPv6 metadata fd00:ec2::254)
		"fd00::/8",
		"fc00::/7",
		// Carrier-grade NAT (some cloud metadata)
		"100.64.0.0/10",
	}
	for _, cidr := range cidrs {
		blockedCIDRs = append(blockedCIDRs, parseCIDR(cidr))
	}
}

// NewSSRFSafeClient creates an http.Client with a custom DialContext that
// resolves hostnames to IPs and blocks dangerous ranges BEFORE connecting.
// This defeats DNS rebinding attacks.
func NewSSRFSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}

			addrs, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed: %w", err)
			}

			for _, resolved := range addrs {
				ip := net.ParseIP(resolved)
				if ip == nil {
					continue
				}
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("SSRF blocked: %s resolves to %s", host, resolved)
				}
			}

			// Connect to the first non-blocked resolved address
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{Transport: transport, Timeout: timeout}
}

// isBlockedIP returns true for IPs that must not be accessed by declarative sources.
func isBlockedIP(ip net.IP) bool {
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// parseCIDR parses a CIDR string and panics on invalid input.
// Only used at init time with hardcoded values.
func parseCIDR(s string) net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR %q: %v", s, err))
	}
	return *cidr
}
