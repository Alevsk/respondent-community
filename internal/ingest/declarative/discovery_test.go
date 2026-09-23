package declarative

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestDiscoveryRejectsInvalidSourceConfigurations(t *testing.T) {
	for _, discovery := range []string{
		"{srv_name: _api._tcp.example.com, allowed_suffix: localhost, port: 443, cache_ttl: 1h}",
		"{srv_name: _api._tcp.example.com, allowed_suffix: api.example.com, port: 80, cache_ttl: 1h}",
		"{srv_name: _api._tcp.example.com, allowed_suffix: api.example.com, port: 443, cache_ttl: 0s}",
	} {
		yaml := strings.Replace(validSourceYAML, "  type: http_poll", "  type: http_poll\n  discovery: "+discovery, 1)
		if _, err := loadMediaTestSource(t, yaml); err == nil {
			t.Errorf("accepted %s", discovery)
		}
	}
}

type fakeSRVResolver struct {
	records []*net.SRV
	calls   int
	err     error
}

func (r *fakeSRVResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	r.calls++
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	return "", r.records, r.err
}

func TestDiscoveryCachesAndValidatesTargets(t *testing.T) {
	now := time.Unix(100, 0)
	dns := &fakeSRVResolver{records: []*net.SRV{
		{Target: "good.api.example.com.", Port: 443, Priority: 1, Weight: 1},
		{Target: "api.example.com.evil.test.", Port: 443},
		{Target: "bad.api.example.com.", Port: 80},
	}}
	r := newEndpointResolver(&DiscoverySpec{SRVName: "_api._tcp.example.com", AllowedSuffix: "api.example.com", Port: 443, CacheTTL: Duration{time.Minute}}, dns, func() time.Time { return now })
	for i := 0; i < 2; i++ {
		origins, err := r.Origins(context.Background())
		if err != nil || len(origins) != 1 || origins[0] != "https://good.api.example.com" {
			t.Fatalf("origins=%v err=%v", origins, err)
		}
	}
	if dns.calls != 1 {
		t.Fatalf("DNS calls=%d", dns.calls)
	}
	now = now.Add(2 * time.Minute)
	dns.err = fmt.Errorf("DNS unavailable")
	if _, err := r.Origins(context.Background()); err == nil {
		t.Fatal("expired discovery survived DNS failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Origins(ctx); err == nil {
		t.Fatal("canceled discovery succeeded")
	}
}

type mediaRoundTripper func(*http.Request) (*http.Response, error)

func (f mediaRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDiscoveryTransportRotatesHostsPreservingPathAndQuery(t *testing.T) {
	var hosts []string
	client := &http.Client{Transport: mediaRoundTripper(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		if r.URL.RequestURI() != "/catalog?offset=0&limit=2" {
			t.Fatalf("path/query changed: %s", r.URL)
		}
		if r.URL.Host == "first.api.example.com" {
			return nil, fmt.Errorf("offline")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})}
	tr := NewHTTPTransport(HTTPTransportConfig{Client: client, Logger: logging.NewNopLogger()})
	tr.discovery = newEndpointResolver(&DiscoverySpec{SRVName: "_api._tcp.example.com", AllowedSuffix: "api.example.com", Port: 443, CacheTTL: Duration{time.Hour}}, &fakeSRVResolver{records: []*net.SRV{{Target: "first.api.example.com.", Port: 443, Priority: 1}, {Target: "second.api.example.com.", Port: 443, Priority: 2}}}, time.Now)
	b, _, err := tr.Fetch(context.Background(), "GET", "https://api.example.com/catalog?offset=0&limit=2")
	if err != nil || string(b) != "[]" || len(hosts) != 2 {
		t.Fatalf("hosts=%v body=%s err=%v", hosts, b, err)
	}
}

func TestDiscoveredPaginationDoesNotPublishPartialOrMixMirrors(t *testing.T) {
	var requests []string
	client := &http.Client{Transport: mediaRoundTripper(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Host+":"+r.URL.Query().Get("offset"))
		if r.URL.Host == "first.api.example.com" && r.URL.Query().Get("offset") == "1" {
			return nil, fmt.Errorf("page failed")
		}
		body := `[]`
		if r.URL.Query().Get("offset") == "0" {
			body = `[{"id":"` + r.URL.Host + `","lat":1.0,"lon":2.0,"alt":0.0,"time":1,"speed":0.0}]`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	yaml := strings.Replace(validSourceYAML, "records_path: \"data.items\"", "records_path: \"\"", 1)
	yaml = strings.Replace(yaml, "  type: http_poll", `  type: http_poll
  pagination:
    type: offset
    page_param: offset
    size_param: limit
    size: 1
    max_pages: 3
    stop_when: 'size(records) == 0'`, 1)
	cs, err := loadMediaTestSource(t, yaml)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), client)
	if err != nil {
		t.Fatal(err)
	}
	a.transport.(*HTTPTransport).discovery = newEndpointResolver(&DiscoverySpec{SRVName: "_api._tcp.example.com", AllowedSuffix: "api.example.com", Port: 443, CacheTTL: Duration{time.Hour}}, &fakeSRVResolver{records: []*net.SRV{{Target: "first.api.example.com.", Port: 443, Priority: 1}, {Target: "second.api.example.com.", Port: 443, Priority: 2}}}, time.Now)
	if err := a.fetchAndProcess(context.Background()); err != nil {
		t.Fatal(err)
	}
	entities, _, err := a.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 || entities[0].ExternalID != "second.api.example.com" {
		t.Fatalf("mixed/partial entities: %+v", entities)
	}
	if strings.Join(requests, ",") != "first.api.example.com:0,first.api.example.com:1,second.api.example.com:0,second.api.example.com:1" {
		t.Fatalf("requests=%v", requests)
	}
}
