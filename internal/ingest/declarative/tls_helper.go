package declarative

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// buildTLSConfig creates a *tls.Config from a TLSSpec.
// Returns nil, nil if spec is nil (no TLS configuration requested).
func buildTLSConfig(spec *TLSSpec) (*tls.Config, error) {
	if spec == nil {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: spec.InsecureSkipVerify, // #nosec G402 -- user-controlled for dev/test environments
		MinVersion:         tls.VersionTLS12,
	}

	if spec.CACert != "" {
		caCert, err := os.ReadFile(spec.CACert)
		if err != nil {
			return nil, fmt.Errorf("read CA cert %q: %w", spec.CACert, err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert from %q", spec.CACert)
		}
		tlsCfg.RootCAs = caCertPool
	}

	if spec.ClientCert != "" || spec.ClientKey != "" {
		if spec.ClientCert == "" || spec.ClientKey == "" {
			return nil, fmt.Errorf("both client_cert and client_key must be provided together")
		}
		cert, err := tls.LoadX509KeyPair(spec.ClientCert, spec.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
