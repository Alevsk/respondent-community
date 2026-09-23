package declarative

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiscoverySpec declares optional DNS SRV origin discovery for an HTTP source.
//
// Discovery replaces only the *origin* of the declared request URL. The
// configured path, query, headers, response limits and rate budget stay
// authoritative, so a source keeps one contract whichever mirror serves it.
type DiscoverySpec struct {
	// SRVName is the full SRV record name, e.g. "_api._tcp.radio-browser.info".
	SRVName string `yaml:"srv_name"`
	// AllowedSuffix bounds which resolved targets may be used. A target is
	// accepted only when it equals the suffix or is a subdomain of it.
	AllowedSuffix string `yaml:"allowed_suffix"`
	// Port is the only SRV port accepted. HTTPS discovery means 443.
	Port int `yaml:"port"`
	// CacheTTL bounds how long a resolved mirror list is reused.
	CacheTTL Duration `yaml:"cache_ttl"`
}

const (
	minDiscoveryCacheTTL = time.Minute
	maxDiscoveryCacheTTL = 24 * time.Hour
)

var srvNameRE = regexp.MustCompile(`^_[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\._(tcp|udp)\.([a-z0-9-]+\.)+[a-z]{2,}$`)

// validateDiscovery rejects discovery declarations that could point a source at
// an unintended origin. It runs at source load, before any network call.
func validateDiscovery(d *DiscoverySpec) error {
	if d == nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d.SRVName), "."))
	if !srvNameRE.MatchString(name) {
		return fmt.Errorf("transport.discovery: srv_name %q must look like _service._tcp.example.com", d.SRVName)
	}
	if _, ok := canonicalDNSHost(d.AllowedSuffix); !ok {
		return fmt.Errorf("transport.discovery: allowed_suffix %q must be a public DNS suffix", d.AllowedSuffix)
	}
	if d.Port != 443 {
		return fmt.Errorf("transport.discovery: port must be 443, got %d", d.Port)
	}
	if d.CacheTTL.Duration < minDiscoveryCacheTTL || d.CacheTTL.Duration > maxDiscoveryCacheTTL {
		return fmt.Errorf("transport.discovery: cache_ttl must be between %s and %s, got %s",
			minDiscoveryCacheTTL, maxDiscoveryCacheTTL, d.CacheTTL.Duration)
	}
	return nil
}

// srvResolver is the injected DNS seam. *net.Resolver satisfies it.
type srvResolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

// endpointResolver resolves and caches the HTTPS origins a source may use.
// The zero value is not usable; construct one with newEndpointResolver.
type endpointResolver struct {
	spec *DiscoverySpec
	dns  srvResolver
	now  func() time.Time

	mu      sync.Mutex
	origins []string
	expires time.Time
}

func newEndpointResolver(spec *DiscoverySpec, dns srvResolver, now func() time.Time) *endpointResolver {
	if now == nil {
		now = time.Now
	}
	return &endpointResolver{spec: spec, dns: dns, now: now}
}

// Origins returns the usable HTTPS origins in preference order (SRV priority
// ascending, then weight descending). An expired cache is never served: a
// failed refresh is an error, not a silent fallback to stale mirrors.
func (r *endpointResolver) Origins(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	if len(r.origins) > 0 && now.Before(r.expires) {
		return append([]string(nil), r.origins...), nil
	}

	service, proto, name, err := splitSRVName(r.spec.SRVName)
	if err != nil {
		return nil, err
	}

	_, records, err := r.dns.LookupSRV(ctx, service, proto, name)
	if err != nil {
		r.origins, r.expires = nil, time.Time{}
		return nil, fmt.Errorf("lookup SRV %q: %w", r.spec.SRVName, err)
	}

	origins := r.acceptableOrigins(records)
	if len(origins) == 0 {
		r.origins, r.expires = nil, time.Time{}
		return nil, fmt.Errorf("lookup SRV %q: no target matched suffix %q on port %d",
			r.spec.SRVName, r.spec.AllowedSuffix, r.spec.Port)
	}

	r.origins = origins
	r.expires = now.Add(r.spec.CacheTTL.Duration)
	return append([]string(nil), origins...), nil
}

// acceptableOrigins filters SRV answers down to the declared suffix and port,
// then orders them by priority (ascending) and weight (descending).
func (r *endpointResolver) acceptableOrigins(records []*net.SRV) []string {
	suffix, ok := canonicalDNSHost(r.spec.AllowedSuffix)
	if !ok {
		return nil
	}

	type candidate struct {
		host     string
		priority uint16
		weight   uint16
	}

	accepted := make([]candidate, 0, len(records))
	for _, rec := range records {
		if rec == nil || int(rec.Port) != r.spec.Port {
			continue
		}
		// LookupSRV is an injected seam, so a target is untrusted text until the
		// shared host policy says it is a public DNS name. That also rules out
		// address literals, embedded paths, ports and illegal label characters.
		host, ok := canonicalDNSHost(rec.Target)
		if !ok || (host != suffix && !strings.HasSuffix(host, "."+suffix)) {
			continue
		}
		accepted = append(accepted, candidate{host: host, priority: rec.Priority, weight: rec.Weight})
	}

	sort.SliceStable(accepted, func(i, j int) bool {
		if accepted[i].priority != accepted[j].priority {
			return accepted[i].priority < accepted[j].priority
		}
		if accepted[i].weight != accepted[j].weight {
			return accepted[i].weight > accepted[j].weight
		}
		return accepted[i].host < accepted[j].host
	})

	seen := make(map[string]bool, len(accepted))
	origins := make([]string, 0, len(accepted))
	for _, c := range accepted {
		origin := "https://" + c.host
		if seen[origin] {
			continue
		}
		seen[origin] = true
		origins = append(origins, origin)
	}
	return origins
}

// splitSRVName splits "_api._tcp.example.com" into ("api", "tcp", "example.com")
// for net.Resolver.LookupSRV, which reassembles the record name itself.
func splitSRVName(srvName string) (service, proto, name string, err error) {
	trimmed := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(srvName), "."))
	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "_") || !strings.HasPrefix(parts[1], "_") {
		return "", "", "", fmt.Errorf("invalid SRV name %q", srvName)
	}
	return strings.TrimPrefix(parts[0], "_"), strings.TrimPrefix(parts[1], "_"), parts[2], nil
}

// rewriteOrigin replaces the scheme and host of rawURL with those of origin,
// preserving the declared path and query verbatim.
func rewriteOrigin(rawURL, origin string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse source URL: %w", err)
	}
	o, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("parse discovered origin: %w", err)
	}
	u.Scheme = o.Scheme
	u.Host = o.Host
	u.User = nil
	return u.String(), nil
}
