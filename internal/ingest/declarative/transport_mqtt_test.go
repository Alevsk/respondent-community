package declarative

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func testLogger() *logging.Logger {
	return logging.NewNopLogger()
}

func ptrInt(v int) *int { return &v }

func ptrBool(b bool) *bool { return &b }

func TestMQTTTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	tr := &MQTTTransport{
		msgCh: make(chan []byte, 1),
		done:  make(chan struct{}),
	}

	_, _, err := tr.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Fatalf("expected ErrNotPullBased, got %v", err)
	}
}

func TestMQTTTransport_NewMQTTTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker:   "tcp://localhost:1883",
				ClientID: "my-client",
				Topics: []MQTTTopicSub{
					{Topic: "sensors/+/data", QoS: ptrInt(1)},
				},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if mt.spec.Broker != "tcp://localhost:1883" {
		t.Errorf("expected broker tcp://localhost:1883, got %s", mt.spec.Broker)
	}
	if mt.spec.ClientID != "my-client" {
		t.Errorf("expected client ID my-client, got %s", mt.spec.ClientID)
	}
	if mt.sourceName != "test_mqtt" {
		t.Errorf("expected source name test_mqtt, got %s", mt.sourceName)
	}
}

func TestMQTTTransport_NewMQTTTransport_MissingSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			// MQTT is nil
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for nil MQTT spec, got nil")
	}
	if !strings.Contains(err.Error(), "transport.mqtt configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMQTTTransport_CredentialResolution(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker:   "tcp://localhost:1883",
				Username: "MQTT_USER",
				Password: "MQTT_PASS",
				Topics:   []MQTTTopicSub{{Topic: "test/#"}},
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "MQTT_USER":
			return "resolved-user"
		case "MQTT_PASS":
			return "resolved-pass"
		default:
			return ""
		}
	}

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if mt.username != "resolved-user" {
		t.Errorf("expected username resolved-user, got %s", mt.username)
	}
	if mt.password != "resolved-pass" {
		t.Errorf("expected password resolved-pass, got %s", mt.password)
	}
}

func TestMQTTTransport_DefaultClientID(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_source",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker: "tcp://localhost:1883",
				// ClientID intentionally empty
				Topics: []MQTTTopicSub{{Topic: "test/#"}},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	// The client ID is auto-generated in Connect(), not in the constructor.
	// Verify that spec has empty client ID so Connect() will generate one.
	if mt.spec.ClientID != "" {
		t.Errorf("expected empty client ID in spec, got %s", mt.spec.ClientID)
	}
}

func TestMQTTTransport_DefaultQoS(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker: "tcp://localhost:1883",
				Topics: []MQTTTopicSub{
					{Topic: "sensors/data"}, // no QoS set
				},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if mt.spec.Topics[0].QoS != nil {
		t.Errorf("expected nil QoS (defaults to 0 at subscribe time), got %d", *mt.spec.Topics[0].QoS)
	}
}

func TestMQTTTransport_RecvContextCancellation(t *testing.T) {
	tr := &MQTTTransport{
		msgCh: make(chan []byte, 1),
		done:  make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := tr.Recv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestMQTTTransport_RecvMessage(t *testing.T) {
	tr := &MQTTTransport{
		msgCh: make(chan []byte, 1),
		done:  make(chan struct{}),
	}

	expected := []byte(`{"lat":42.0,"lon":-71.0}`)
	tr.msgCh <- expected

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, err := tr.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != string(expected) {
		t.Errorf("expected %s, got %s", expected, msg)
	}
}

func TestMQTTTransport_RecvChannelClosed(t *testing.T) {
	tr := &MQTTTransport{
		msgCh: make(chan []byte, 1),
		done:  make(chan struct{}),
	}

	close(tr.msgCh)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := tr.Recv(ctx)
	if err == nil {
		t.Fatal("expected error on closed channel, got nil")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' in error, got %v", err)
	}
}

func TestMQTTTransport_CloseIdempotent(t *testing.T) {
	tr := &MQTTTransport{
		sourceName: "test",
		logger:     testLogger(),
		msgCh:      make(chan []byte, 1),
		done:       make(chan struct{}),
		// client is nil -- Close() must handle this
	}

	err := tr.Close()
	if err != nil {
		t.Fatalf("first Close() error: %v", err)
	}

	err = tr.Close()
	if err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestMQTTTransport_CloseWithoutConnect(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker: "tcp://localhost:1883",
				Topics: []MQTTTopicSub{{Topic: "test/#"}},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close without ever calling Connect -- must not panic.
	mt := st.(*MQTTTransport)
	err = mt.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestBuildTLSConfig_NilSpec(t *testing.T) {
	cfg, err := buildTLSConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for nil spec")
	}
}

func TestBuildTLSConfig_InsecureSkipVerify(t *testing.T) {
	spec := &TLSSpec{
		InsecureSkipVerify: true,
	}

	cfg, err := buildTLSConfig(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfig_CACert(t *testing.T) {
	// Generate a self-signed CA cert for testing.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatalf("write CA cert: %v", err)
	}

	spec := &TLSSpec{
		CACert: caPath,
	}

	cfg, err := buildTLSConfig(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
}

func TestBuildTLSConfig_InvalidCACertPath(t *testing.T) {
	spec := &TLSSpec{
		CACert: "/nonexistent/ca.pem",
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error for nonexistent CA cert")
	}
}

func TestBuildTLSConfig_InvalidCACertContent(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "bad-ca.pem")
	if err := os.WriteFile(caPath, []byte("not a cert"), 0o600); err != nil {
		t.Fatalf("write bad cert: %v", err)
	}

	spec := &TLSSpec{
		CACert: caPath,
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error for invalid CA cert content")
	}
}

func TestBuildTLSConfig_ClientCertKeyPair(t *testing.T) {
	// Generate a self-signed cert + key for testing.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client-key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	spec := &TLSSpec{
		ClientCert: certPath,
		ClientKey:  keyPath,
	}

	cfg, err := buildTLSConfig(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
}

func TestBuildTLSConfig_InvalidClientCertPath(t *testing.T) {
	spec := &TLSSpec{
		ClientCert: "/nonexistent/client.pem",
		ClientKey:  "/nonexistent/client-key.pem",
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error for nonexistent client cert")
	}
}

func TestMQTTTransport_RegisteredInFactory(t *testing.T) {
	// Verify the init() function registered "mqtt" in the transport factory.
	transportMu.RLock()
	_, ok := transportConstructors["mqtt"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("mqtt transport not registered in factory")
	}
}

func TestMQTTTransport_MultipleTopics(t *testing.T) {
	def := &SourceDefinition{
		Name: "multi_topic",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker: "tcp://localhost:1883",
				Topics: []MQTTTopicSub{
					{Topic: "sensors/temp", QoS: ptrInt(0)},
					{Topic: "sensors/humidity", QoS: ptrInt(1)},
					{Topic: "sensors/pressure", QoS: ptrInt(2)},
				},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if len(mt.spec.Topics) != 3 {
		t.Errorf("expected 3 topics, got %d", len(mt.spec.Topics))
	}
}

func TestMQTTTransport_CleanSessionDefault(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker: "tcp://localhost:1883",
				// CleanSession not set -- should default to true in Connect()
				Topics: []MQTTTopicSub{{Topic: "test/#"}},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if mt.spec.CleanSession != nil {
		t.Errorf("expected nil CleanSession (defaults to true in Connect), got %v", *mt.spec.CleanSession)
	}
}

func TestMQTTTransport_CleanSessionExplicit(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_mqtt",
		Transport: TransportSpec{
			Type:     "mqtt",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			MQTT: &MQTTSpec{
				Broker:       "tcp://localhost:1883",
				CleanSession: ptrBool(false),
				Topics:       []MQTTTopicSub{{Topic: "test/#"}},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newMQTTTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := st.(*MQTTTransport)
	if mt.spec.CleanSession == nil || *mt.spec.CleanSession != false {
		t.Error("expected CleanSession=false")
	}
}

func TestRandomSuffix(t *testing.T) {
	// Verify randomSuffix returns an 8-char hex string (4 bytes = 8 hex chars).
	s := randomSuffix()
	if len(s) != 8 {
		t.Errorf("expected 8-char suffix, got %d chars: %q", len(s), s)
	}

	// Verify uniqueness across calls.
	s2 := randomSuffix()
	if s == s2 {
		t.Error("expected different suffixes on consecutive calls")
	}
}
