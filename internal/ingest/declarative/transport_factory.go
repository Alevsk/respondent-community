package declarative

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Alevsk/respondent/internal/logging"
)

// TransportConstructor is a function that creates a Transport from a source definition.
type TransportConstructor func(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error)

var (
	transportMu           sync.RWMutex
	transportConstructors = map[string]TransportConstructor{}
)

// RegisterTransport registers a transport constructor for a given type name.
// This is typically called from init() functions in transport implementation files.
func RegisterTransport(name string, ctor TransportConstructor) {
	transportMu.Lock()
	defer transportMu.Unlock()
	transportConstructors[name] = ctor
}

// NewTransport creates a transport implementation based on the transport.type field.
func NewTransport(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, envResolve EnvResolver, client *http.Client, logger *logging.Logger) (Transport, error) {
	// http_poll is always built-in (not registered via init())
	if def.Transport.Type == "http_poll" {
		return newHTTPPollTransport(def, resolvedHeaders, tokenProvider, client, logger)
	}

	transportMu.RLock()
	ctor, ok := transportConstructors[def.Transport.Type]
	transportMu.RUnlock()

	if ok {
		return ctor(def, resolvedHeaders, tokenProvider, envResolve, logger)
	}

	return nil, fmt.Errorf("unsupported transport type: %q", def.Transport.Type)
}

// newHTTPPollTransport creates the existing HTTP polling transport.
func newHTTPPollTransport(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, client *http.Client, logger *logging.Logger) (Transport, error) {
	return NewHTTPTransport(HTTPTransportConfig{
		Client:           client,
		Headers:          resolvedHeaders,
		TokenProvider:    tokenProvider,
		Retry:            def.Transport.Retry,
		MaxResponseBytes: def.Transport.MaxResponseBytes,
		SourceName:       def.Name,
		Logger:           logger,
	}), nil
}
