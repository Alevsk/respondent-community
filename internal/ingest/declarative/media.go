package declarative

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

type MediaSpec struct {
	ID             string             `yaml:"id"`
	Kind           string             `yaml:"kind"`
	Label          string             `yaml:"label"`
	URLKey         string             `yaml:"url_key"`
	AttributionKey string             `yaml:"attribution_key,omitempty"`
	AllowedOrigins []string           `yaml:"allowed_origins,omitempty"`
	PlaybackAction string             `yaml:"playback_action,omitempty"`
	Snapshot       *SnapshotMediaSpec `yaml:"snapshot,omitempty"`
}

type SnapshotMediaSpec struct {
	RefreshInterval Duration `yaml:"refresh_interval"`
	CacheBustParam  string   `yaml:"cache_bust_param,omitempty"`
}

// MediaActionSpec is a bounded playback notification, never a browser-supplied request.
type MediaActionSpec struct {
	Name   string `yaml:"name"`
	Method string `yaml:"method"`
	Path   string `yaml:"path"` // CEL with metadata: map(string,string)
}

var mediaQueryKeyRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

// validateMedia is shared by ingestion and credential-independent display loading.
func validateMedia(media []MediaSpec, entity, observation map[string]string, actions []MediaActionSpec) error {
	actionNames := make(map[string]bool, len(actions))
	for _, a := range actions {
		if !sourceNameRE.MatchString(a.Name) || actionNames[a.Name] || a.Method != "GET" || strings.TrimSpace(a.Path) == "" {
			return fmt.Errorf("media_actions: invalid or duplicate GET action %q", a.Name)
		}
		actionNames[a.Name] = true
	}
	seen := make(map[string]bool, len(media))
	for _, m := range media {
		if !sourceNameRE.MatchString(m.ID) || seen[m.ID] || strings.TrimSpace(m.Label) == "" {
			return fmt.Errorf("display.media: invalid or duplicate id/label %q", m.ID)
		}
		seen[m.ID] = true
		if m.URLKey == "" || !declaredMetadataKey(m.URLKey, entity, observation) {
			return fmt.Errorf("display.media[%s]: url_key must reference declared metadata", m.ID)
		}
		if m.AttributionKey != "" && !declaredMetadataKey(m.AttributionKey, entity, observation) {
			return fmt.Errorf("display.media[%s]: attribution_key must reference declared metadata", m.ID)
		}
		switch m.Kind {
		case "snapshot":
			if m.Snapshot == nil || m.Snapshot.RefreshInterval.Duration < 5*time.Second || m.Snapshot.RefreshInterval.Duration > time.Hour || m.Snapshot.RefreshInterval.Duration%time.Second != 0 {
				return fmt.Errorf("display.media[%s]: snapshot refresh_interval must be whole seconds between 5s and 1h", m.ID)
			}
			if p := m.Snapshot.CacheBustParam; p != "" && !mediaQueryKeyRE.MatchString(p) {
				return fmt.Errorf("display.media[%s]: invalid cache_bust_param", m.ID)
			}
			if m.PlaybackAction != "" {
				return fmt.Errorf("display.media[%s]: only audio supports playback_action", m.ID)
			}
		case "audio":
			if m.Snapshot != nil {
				return fmt.Errorf("display.media[%s]: audio cannot have snapshot options", m.ID)
			}
		default:
			return fmt.Errorf("display.media[%s]: unsupported kind %q", m.ID, m.Kind)
		}
		if m.PlaybackAction != "" && !actionNames[m.PlaybackAction] {
			return fmt.Errorf("display.media[%s]: undefined playback_action %q", m.ID, m.PlaybackAction)
		}
		for _, origin := range m.AllowedOrigins {
			if _, ok := canonicalMediaOrigin(origin); !ok {
				return fmt.Errorf("display.media[%s]: invalid HTTPS origin %q", m.ID, origin)
			}
		}
	}
	return nil
}

func declaredMetadataKey(key string, entity, observation map[string]string) bool {
	_, e := entity[key]
	_, o := observation[key]
	return e || o
}

// canonicalMediaOrigin validates an allowed-origin declaration and returns it
// in the exact spelling a browser produces for `URL.origin`.
//
// Canonicalizing is not cosmetic: the browser compares the media URL's origin
// against this string literally, so "https://cam.example.com/" or
// "https://cam.example.com:443" would match nothing and silently make every
// media item on the layer unavailable, with no diagnostic anywhere.
func canonicalMediaOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || host == "localhost" || !strings.Contains(host, ".") ||
		strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return "", false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
			netip.MustParsePrefix("100.64.0.0/10").Contains(ip) {
			return "", false
		}
		host = "[" + ip.String() + "]"
		if ip.Is4() {
			host = ip.String()
		}
	}
	// Only a non-default port survives; :443 is implicit in an https origin.
	if port := u.Port(); port != "" && port != "443" {
		host += ":" + port
	}
	return "https://" + host, true
}

func mediaSpecToDomain(m MediaSpec) domain.MediaConfig {
	d := domain.MediaConfig{ID: m.ID, Kind: m.Kind, Label: m.Label, URLKey: m.URLKey, AttributionKey: m.AttributionKey, PlaybackAction: m.PlaybackAction}
	for _, origin := range m.AllowedOrigins {
		if canonical, ok := canonicalMediaOrigin(origin); ok {
			d.AllowedOrigins = append(d.AllowedOrigins, canonical)
		}
	}
	if m.Snapshot != nil {
		d.Snapshot = &domain.SnapshotMediaConfig{RefreshIntervalSeconds: int32(m.Snapshot.RefreshInterval.Duration / time.Second), CacheBustParam: m.Snapshot.CacheBustParam}
	}
	return d
}
