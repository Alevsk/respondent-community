package declarative

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCert creates a self-signed certificate for testing.
// Returns paths to the cert and key files in a temp dir.
func generateTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	dir := t.TempDir()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	// Write cert
	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode cert PEM: %v", err)
	}
	_ = certOut.Close()

	// Write key
	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}); err != nil {
		t.Fatalf("encode key PEM: %v", err)
	}
	_ = keyOut.Close()

	return certFile, keyFile
}

func TestBuildTLSConfig_WithClientCertPair(t *testing.T) {
	certFile, keyFile := generateTestCert(t)

	spec := &TLSSpec{
		ClientCert: certFile,
		ClientKey:  keyFile,
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

func TestBuildTLSConfig_OnlyClientCertMissingKey(t *testing.T) {
	certFile, _ := generateTestCert(t)

	// Only cert, no key
	spec := &TLSSpec{
		ClientCert: certFile,
		// ClientKey intentionally missing
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error when ClientCert is set but ClientKey is empty, got nil")
	}
}

func TestBuildTLSConfig_OnlyClientKeyMissingCert(t *testing.T) {
	_, keyFile := generateTestCert(t)

	// Only key, no cert
	spec := &TLSSpec{
		ClientKey: keyFile,
		// ClientCert intentionally missing
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error when ClientKey is set but ClientCert is empty, got nil")
	}
}

func TestBuildTLSConfig_InvalidClientCertPairContent(t *testing.T) {
	dir := t.TempDir()
	// Create mismatched cert/key pair with invalid content
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	if err := os.WriteFile(certFile, []byte("not-a-cert"), 0644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("not-a-key"), 0644); err != nil {
		t.Fatalf("write key: %v", err)
	}

	spec := &TLSSpec{
		ClientCert: certFile,
		ClientKey:  keyFile,
	}

	_, err := buildTLSConfig(spec)
	if err == nil {
		t.Fatal("expected error for invalid cert/key pair, got nil")
	}
}

func TestBuildTLSConfig_EmptySpec(t *testing.T) {
	spec := &TLSSpec{}
	cfg, err := buildTLSConfig(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be false by default")
	}
}
