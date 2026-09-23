package declarative

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// hostKind reports what a host string turned out to be once normalised.
type hostKind int

const (
	// hostInvalid means the string is not a host Respondent will contact.
	hostInvalid hostKind = iota
	// hostDNSName is a public DNS name.
	hostDNSName
	// hostIPLiteral is a global-unicast address literal.
	hostIPLiteral
)

// maxHostLen is the DNS limit on a fully qualified name.
const maxHostLen = 253

// dnsLabelRE matches one already-lowercased DNS label.
var dnsLabelRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// canonicalHost normalises a host and reports what it is.
//
// Every place that decides whether Respondent may talk to a host goes through
// here — declared media origins, a discovery suffix, and the targets a SRV
// lookup hands back. Those three used to screen hosts separately and disagreed
// about the edges, which is the only reason a form like "127.1" could be
// admitted in one of them.
//
// The returned host is the canonical spelling: lowercased, no trailing dot, and
// IPv6 bracketed, so a caller can compare it against a browser's `URL.origin`
// or use it to build one.
func canonicalHost(raw string) (string, hostKind) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > maxHostLen {
		return "", hostInvalid
	}

	// Bracketed IPv6 arrives from URL parsing already stripped, but a caller may
	// also pass the bracketed form directly.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicAddr(ip) {
			return "", hostInvalid
		}
		ip = ip.Unmap()
		if ip.Is4() {
			return ip.String(), hostIPLiteral
		}
		return "[" + ip.String() + "]", hostIPLiteral
	}

	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		// A single label is either a local name or a bare integer; neither is a
		// host on the public internet.
		return "", hostInvalid
	}
	for _, label := range labels {
		if !dnsLabelRE.MatchString(label) {
			return "", hostInvalid
		}
	}

	// A host whose last label is numeric is parsed as an IPv4 address by the URL
	// standard, which is how "127.1", "0177.0.0.1" and "2130706433" all reach
	// loopback. netip.ParseAddr rejects those spellings, so without this rule
	// they would slip through as ordinary names.
	if last := labels[len(labels)-1]; isNumericLabel(last) {
		return "", hostInvalid
	}

	switch last := labels[len(labels)-1]; last {
	case "localhost", "local", "internal", "home", "lan", "invalid", "test", "example", "onion":
		return "", hostInvalid
	}

	return host, hostDNSName
}

// canonicalDNSHost is canonicalHost restricted to names. Discovery uses it:
// a SRV target and an allowed suffix are names by definition, and an address
// literal in either position means the answer was not what we asked for.
func canonicalDNSHost(raw string) (string, bool) {
	host, kind := canonicalHost(raw)
	return host, kind == hostDNSName
}

// isNumericLabel reports whether a label would be read as a number — decimal,
// octal or hexadecimal — by an IPv4 parser.
func isNumericLabel(label string) bool {
	if label == "" {
		return false
	}
	if _, err := strconv.ParseUint(label, 0, 64); err == nil {
		return true
	}
	// strconv rejects an overlong integer, but a parser that saturates would
	// still read it as a number rather than a name.
	for _, r := range label {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// publicAddr reports whether an address literal is one we will contact.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	// Carrier-grade NAT and the benchmarking range are routable-looking but are
	// not destinations on the public internet.
	for _, prefix := range []string{"100.64.0.0/10", "198.18.0.0/15", "192.0.0.0/24"} {
		if netip.MustParsePrefix(prefix).Contains(ip) {
			return false
		}
	}
	return true
}
