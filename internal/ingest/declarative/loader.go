package declarative

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/cel-go/cel"
	"gopkg.in/yaml.v3"

	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// v2OnlyFeatures lists features gated behind schema_version >= 2.
// array_columns is NOT gated: it's a parser convenience, not a structural feature.

// EnvResolver resolves an environment variable name to its value.
// Returns empty string if the variable is not set.
type EnvResolver func(name string) string

// Loader loads, validates, and compiles declarative source definitions from YAML files.
type Loader struct {
	compiler       *CELCompiler
	validate       *validator.Validate
	envResolve     EnvResolver
	devMode        bool
	logger         *logging.Logger
	schemaRegistry *schema.Registry
}

// NewLoader creates a Loader with the given CEL compiler, env resolver, and dev mode flag.
// devMode=true allows http:// URLs (normally only https:// is allowed).
// envResolve is called to resolve environment variable references in URLs and auth specs.
func NewLoader(compiler *CELCompiler, envResolve EnvResolver, devMode bool, logger *logging.Logger) (*Loader, error) {
	v, err := NewValidator()
	if err != nil {
		return nil, fmt.Errorf("create validator: %w", err)
	}

	return &Loader{
		compiler:       compiler,
		validate:       v,
		envResolve:     envResolve,
		devMode:        devMode,
		logger:         logger.WithSource("declarative-loader"),
		schemaRegistry: schema.NewRegistry(),
	}, nil
}

// SchemaRegistry returns the schema registry used to compile and cache
// JSON Schemas from AI operation output_schema fields.
func (l *Loader) SchemaRegistry() *schema.Registry {
	return l.schemaRegistry
}

// LoadDir loads all *.yaml and *.yml files from the given directory.
// Loading is best-effort: a bad file logs an error and is skipped.
// Returns all successfully compiled sources.
func (l *Loader) LoadDir(dir string) ([]*CompiledSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sources directory %q: %w", dir, err)
	}

	var sources []*CompiledSource
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		cs, err := l.LoadFile(path)
		if err != nil {
			l.logger.Error("failed to load source definition",
				logging.String("file", path),
				logging.Err("error", err),
			)
			continue
		}

		// Skip disabled sources
		if !cs.definition.IsEnabled() {
			l.logger.Info("skipping disabled source",
				logging.String("file", path),
				logging.String("source_name", cs.definition.Name),
			)
			continue
		}

		sources = append(sources, cs)
		l.logger.Info("loaded source definition",
			logging.String("file", path),
			logging.String("source_name", cs.definition.Name),
			logging.String("source_type", cs.definition.SourceType),
			logging.String("layer_type", cs.definition.LayerType),
			logging.Any("dry_run", cs.definition.DryRun),
		)
	}

	return sources, nil
}

// LoadFile loads a single YAML source definition file.
func (l *Loader) LoadFile(path string) (*CompiledSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	var def SourceDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML %q: %w", path, err)
	}

	// Validate struct fields
	if err := l.validate.Struct(def); err != nil {
		return nil, fmt.Errorf("validate %q: %w", path, err)
	}
	if err := validateMedia(def.Display.Media, def.Entity.Metadata, def.Observation.Metadata, def.MediaActions); err != nil {
		return nil, fmt.Errorf("validate %q: %w", path, err)
	}
	if err := validateDiscovery(def.Transport.Discovery); err != nil {
		return nil, fmt.Errorf("validate %q: %w", path, err)
	}

	// Cross-field: dedupe mode requires a non-empty content_hash expression.
	if def.Recording.Mode == "dedupe" && strings.TrimSpace(def.Observation.ContentHash) == "" {
		return nil, fmt.Errorf("validate %q: recording.mode is 'dedupe' but observation.content_hash is empty", path)
	}

	// Default entity_type to "geo_entity" when omitted.
	if def.EntityType == "" {
		def.EntityType = string(domain.EntityTypeGeo)
	}

	// Cross-field: geo_entity requires observation.latitude and observation.longitude.
	if def.EntityType == string(domain.EntityTypeGeo) {
		if strings.TrimSpace(def.Observation.Latitude) == "" {
			return nil, fmt.Errorf("validate %q: observation.latitude is required for entity_type %q", path, def.EntityType)
		}
		if strings.TrimSpace(def.Observation.Longitude) == "" {
			return nil, fmt.Errorf("validate %q: observation.longitude is required for entity_type %q", path, def.EntityType)
		}
	}

	// Log info when loading a global indicator source.
	if def.EntityType == string(domain.EntityTypeIndicator) {
		l.logger.Info("loading global indicator source",
			logging.String("file", path),
			logging.String("source_name", def.Name),
		)
	}

	// Conditional transport validation (URL, interval, sub-spec requirements)
	if err := l.validateTransportSpec(&def, path); err != nil {
		return nil, err
	}

	// Gate v2 features behind schema_version >= 2.
	if def.SchemaVersion < 2 {
		if def.Filtering != "" {
			return nil, fmt.Errorf("validate %q: 'filtering' requires schema_version >= 2", path)
		}
		if def.Backfill != nil {
			return nil, fmt.Errorf("validate %q: 'backfill' requires schema_version >= 2", path)
		}
		if def.EntityCache != nil {
			return nil, fmt.Errorf("validate %q: 'entity_cache' requires schema_version >= 2", path)
		}
		if def.Transport.Spatial != nil {
			return nil, fmt.Errorf("validate %q: 'transport.spatial' requires schema_version >= 2", path)
		}
		if def.Transport.OnDemandURL != "" {
			return nil, fmt.Errorf("validate %q: 'transport.on_demand_url' requires schema_version >= 2", path)
		}
		if def.Parser.ObjectToRecords {
			return nil, fmt.Errorf("validate %q: 'parser.object_to_records' requires schema_version >= 2", path)
		}
		if def.Parser.ArrayOfArrays {
			return nil, fmt.Errorf("validate %q: 'parser.array_of_arrays' requires schema_version >= 2", path)
		}
	}

	// Default HTTP method to GET if not specified.
	if def.Transport.Method == "" {
		def.Transport.Method = "GET"
	}

	// Only validate URL for transports that use the URL field
	switch def.Transport.Type {
	case "http_poll", "websocket", "sse":
		// Resolve ${ENV_VAR} references in the URL (e.g. FIRMS embeds API key in URL path).
		def.Transport.URL = os.Expand(def.Transport.URL, l.envResolve)

		// Validate URL scheme (skip full validation for template URLs with {lat}/{lon})
		urlToValidate := def.Transport.URL
		if def.Transport.Spatial != nil {
			// Template URLs contain {lat}, {lon} etc. -- validate scheme only
			urlToValidate = strings.NewReplacer(
				"{lat}", "0", "{lon}", "0",
				"{lat_min}", "0", "{lat_max}", "0",
				"{lon_min}", "0", "{lon_max}", "0",
			).Replace(urlToValidate)
		}
		if err := l.validateURL(urlToValidate); err != nil {
			return nil, fmt.Errorf("validate URL in %q: %w", path, err)
		}
	default:
		// Non-URL transports: still expand env vars if URL is present
		if def.Transport.URL != "" {
			def.Transport.URL = os.Expand(def.Transport.URL, l.envResolve)
		}
	}

	// Resolve on_demand_url env vars if present
	if def.Transport.OnDemandURL != "" {
		def.Transport.OnDemandURL = os.Expand(def.Transport.OnDemandURL, l.envResolve)
	}

	// Resolve ${ENV_VAR} references in WebSocket subscribe messages.
	if def.Transport.WebSocket != nil {
		for i, msg := range def.Transport.WebSocket.SubscribeMessages {
			def.Transport.WebSocket.SubscribeMessages[i] = os.Expand(msg, l.envResolve)
		}
	}

	// Resolve inline auth (skip for disabled sources — env vars may not be set)
	var resolvedHeaders map[string]string
	var tokenProvider TokenProvider
	if def.IsEnabled() {
		tokenProvider, resolvedHeaders, err = l.buildTokenProvider(def.Transport.Auth, def.Transport.Headers, def.Name)
		if err != nil {
			return nil, fmt.Errorf("resolve auth in %q: %w", path, err)
		}
	}

	// Validate lookup tables
	for i, lt := range def.LookupTables {
		hasEntries := len(lt.Entries) > 0
		hasFile := lt.File != ""
		if hasEntries == hasFile {
			return nil, fmt.Errorf("validate %q: lookup_tables[%d] %q: exactly one of 'entries' or 'file' must be set", path, i, lt.Name)
		}
		if hasFile && lt.Format == "" {
			return nil, fmt.Errorf("validate %q: lookup_tables[%d] %q: 'format' is required when 'file' is set", path, i, lt.Name)
		}
	}

	// Derive sourcesDir from the file path for resolving lookup file references.
	sourcesDir := filepath.Dir(path)

	// Compile all CEL expressions
	cs, err := l.compileCEL(&def, resolvedHeaders, tokenProvider, sourcesDir)
	if err != nil {
		return nil, fmt.Errorf("compile CEL in %q: %w", path, err)
	}

	// Validate AI operations if present and enabled.
	if def.AI != nil && def.AI.Enabled {
		if err := def.AI.Validate(l.validate); err != nil {
			return nil, fmt.Errorf("validate ai in %q: %w", path, err)
		}
		for i, op := range def.AI.Operations {
			// Compile CEL filter expressions for AI operations.
			if op.Filter != "" {
				if _, err := l.compiler.CompileExpression(strings.TrimSpace(op.Filter)); err != nil {
					return nil, fmt.Errorf("compile ai.operations[%d].filter in %q: %w", i, path, err)
				}
			}
			// Register output schemas in the schema registry.
			if op.OutputSchema != nil {
				schemaKey := def.Name + "." + op.Name
				if err := l.schemaRegistry.RegisterFromYAML(schemaKey, op.OutputSchema); err != nil {
					return nil, fmt.Errorf("register ai.operations[%d].output_schema in %q: %w", i, path, err)
				}
			}
		}
	}

	return cs, nil
}

// validateURL checks that the URL scheme is allowed.
func (l *Loader) validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
		return nil
	case "http":
		if l.devMode {
			return nil
		}
		return fmt.Errorf("http URLs are not allowed in production (use https or enable dev mode)")
	case "ws", "wss":
		return nil // WebSocket schemes always allowed
	default:
		return fmt.Errorf("unsupported URL scheme %q", scheme)
	}
}

// buildTokenProvider builds a TokenProvider and static headers from the auth spec.
// Static auth types (bearer, api_key) return a staticTokenProvider.
// OAuth2 returns an oauth2TokenProvider that manages token lifecycle at runtime.
// The returned headers map contains only non-auth static headers.
func (l *Loader) buildTokenProvider(auth *AuthSpec, staticHeaders map[string]string, sourceName string) (TokenProvider, map[string]string, error) {
	headers := make(map[string]string)
	for k, v := range staticHeaders {
		headers[k] = v
	}

	if auth == nil {
		return nil, headers, nil
	}

	switch auth.Type {
	case "bearer":
		envVal := l.envResolve(auth.EnvVar)
		if envVal == "" {
			return nil, nil, fmt.Errorf("environment variable %q for auth is empty or not set", auth.EnvVar)
		}
		return &staticTokenProvider{
			headerName:  "Authorization",
			headerValue: "Bearer " + envVal,
		}, headers, nil

	case "api_key":
		if auth.Header == "" {
			return nil, nil, fmt.Errorf("api_key auth requires a header name")
		}
		envVal := l.envResolve(auth.EnvVar)
		if envVal == "" {
			return nil, nil, fmt.Errorf("environment variable %q for auth is empty or not set", auth.EnvVar)
		}
		return &staticTokenProvider{
			headerName:  auth.Header,
			headerValue: envVal,
		}, headers, nil

	case "oauth2":
		if auth.OAuth2 == nil {
			return nil, nil, fmt.Errorf("oauth2 auth type requires 'oauth2' configuration block")
		}
		if err := l.validateOAuth2Spec(auth.OAuth2); err != nil {
			return nil, nil, err
		}
		creds, err := l.resolveOAuth2Credentials(auth.OAuth2.Credentials, auth.OAuth2.GrantType)
		if err != nil {
			return nil, nil, err
		}
		provider := newOAuth2TokenProvider(auth.OAuth2, creds, sourceName, l.logger)
		return provider, headers, nil

	default:
		return nil, nil, fmt.Errorf("unsupported auth type %q", auth.Type)
	}
}

// validateOAuth2Spec validates the OAuth2 configuration block.
func (l *Loader) validateOAuth2Spec(spec *OAuth2Spec) error {
	if spec.TokenURL == "" {
		return fmt.Errorf("oauth2.token_url is required")
	}
	switch spec.GrantType {
	case "password":
		if spec.Credentials.Username.Value == "" && spec.Credentials.Username.EnvVar == "" {
			return fmt.Errorf("password grant requires username credential")
		}
		if spec.Credentials.Password.Value == "" && spec.Credentials.Password.EnvVar == "" {
			return fmt.Errorf("password grant requires password credential")
		}
	case "client_credentials":
		if spec.Credentials.ClientID.Value == "" && spec.Credentials.ClientID.EnvVar == "" {
			return fmt.Errorf("client_credentials grant requires client_id credential")
		}
		if spec.Credentials.ClientSecret.Value == "" && spec.Credentials.ClientSecret.EnvVar == "" {
			return fmt.Errorf("client_credentials grant requires client_secret credential")
		}
	default:
		return fmt.Errorf("unsupported grant_type: %q", spec.GrantType)
	}
	return nil
}

// resolveOAuth2Credentials resolves OAuth2 credential values from env vars or literals.
// Only fields required by the grant type are resolved; missing required fields return an error.
func (l *Loader) resolveOAuth2Credentials(creds OAuth2Credentials, grantType string) (resolvedOAuth2Credentials, error) {
	resolve := func(v OAuth2Value, fieldName string) (string, error) {
		if v.EnvVar != "" {
			val := l.envResolve(v.EnvVar)
			if val == "" {
				return "", fmt.Errorf("environment variable %q for oauth2 %s is empty or not set", v.EnvVar, fieldName)
			}
			return val, nil
		}
		return v.Value, nil
	}

	var r resolvedOAuth2Credentials
	var err error

	switch grantType {
	case "password":
		r.Username, err = resolve(creds.Username, "username")
		if err != nil {
			return r, err
		}
		r.Password, err = resolve(creds.Password, "password")
		if err != nil {
			return r, err
		}
		// client_id is optional for password grant (some providers require it, some don't)
		r.ClientID, _ = resolve(creds.ClientID, "client_id")

	case "client_credentials":
		r.ClientID, err = resolve(creds.ClientID, "client_id")
		if err != nil {
			return r, err
		}
		r.ClientSecret, err = resolve(creds.ClientSecret, "client_secret")
		if err != nil {
			return r, err
		}

	default:
		return r, fmt.Errorf("unsupported grant_type: %q", grantType)
	}

	return r, nil
}

// compileCEL compiles all CEL expressions from a source definition into a CompiledSource.
// sourcesDir is used to resolve file-based lookup table paths.
func (l *Loader) compileCEL(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, sourcesDir string) (*CompiledSource, error) {
	cs := &CompiledSource{
		clock:           l.compiler.Clock(),
		definition:      def,
		resolvedHeaders: resolvedHeaders,
		tokenProvider:   tokenProvider,
		envResolve:      l.envResolve,
		entityMeta:      make(map[string]cel.Program),
	}

	// Load lookup tables if defined
	if len(def.LookupTables) > 0 {
		tables, err := LoadLookupTables(def.LookupTables, sourcesDir)
		if err != nil {
			return nil, fmt.Errorf("load lookup tables: %w", err)
		}
		cs.lookupTables = tables
	}

	// Choose compile function based on whether lookups are present.
	// When tables exist, use an extended CEL environment with lookup functions.
	compile := l.compiler.CompileExpression
	if cs.lookupTables != nil {
		compile = func(expr string) (cel.Program, error) {
			return l.compiler.CompileExpressionWithLookups(expr, cs.lookupTables)
		}
	}

	// Compile filter (optional)
	if def.Filter != "" {
		prg, err := compile(strings.TrimSpace(def.Filter))
		if err != nil {
			return nil, fmt.Errorf("compile filter: %w", err)
		}
		cs.filter = prg
	}

	// Compile entity mappings
	var err error
	cs.entityID, err = compile(strings.TrimSpace(def.Entity.ExternalID))
	if err != nil {
		return nil, fmt.Errorf("compile entity.external_id: %w", err)
	}

	cs.entityName, err = compile(strings.TrimSpace(def.Entity.Name))
	if err != nil {
		return nil, fmt.Errorf("compile entity.name: %w", err)
	}

	for k, expr := range def.Entity.Metadata {
		prg, err := compile(strings.TrimSpace(expr))
		if err != nil {
			return nil, fmt.Errorf("compile entity.metadata[%s]: %w", k, err)
		}
		cs.entityMeta[k] = prg
	}

	// Compile observation mappings.
	// Global indicator entities have no lat/lon; skip compilation when expressions are empty.
	if strings.TrimSpace(def.Observation.Latitude) != "" {
		cs.observationLat, err = compile(strings.TrimSpace(def.Observation.Latitude))
		if err != nil {
			return nil, fmt.Errorf("compile observation.latitude: %w", err)
		}
	}

	if strings.TrimSpace(def.Observation.Longitude) != "" {
		cs.observationLon, err = compile(strings.TrimSpace(def.Observation.Longitude))
		if err != nil {
			return nil, fmt.Errorf("compile observation.longitude: %w", err)
		}
	}

	if def.Observation.Altitude != "" {
		cs.observationAlt, err = compile(strings.TrimSpace(def.Observation.Altitude))
		if err != nil {
			return nil, fmt.Errorf("compile observation.altitude: %w", err)
		}
	}

	cs.observationTS, err = compile(strings.TrimSpace(def.Observation.Timestamp))
	if err != nil {
		return nil, fmt.Errorf("compile observation.timestamp: %w", err)
	}

	if def.Observation.EventTime != "" {
		cs.observationEventTime, err = compile(strings.TrimSpace(def.Observation.EventTime))
		if err != nil {
			return nil, fmt.Errorf("compile observation.event_time: %w", err)
		}
	}

	if def.Observation.EventEnd != "" {
		cs.observationEventEnd, err = compile(strings.TrimSpace(def.Observation.EventEnd))
		if err != nil {
			return nil, fmt.Errorf("compile observation.event_end: %w", err)
		}
	}

	// Compile velocity mappings
	if len(def.Observation.Velocity) > 0 {
		cs.observationVelocity = make(map[string]cel.Program)
		for k, expr := range def.Observation.Velocity {
			prg, err := compile(strings.TrimSpace(expr))
			if err != nil {
				return nil, fmt.Errorf("compile observation.velocity[%s]: %w", k, err)
			}
			cs.observationVelocity[k] = prg
		}
	}

	// Compile observation metadata
	if len(def.Observation.Metadata) > 0 {
		cs.observationMeta = make(map[string]cel.Program)
		for k, expr := range def.Observation.Metadata {
			prg, err := compile(strings.TrimSpace(expr))
			if err != nil {
				return nil, fmt.Errorf("compile observation.metadata[%s]: %w", k, err)
			}
			cs.observationMeta[k] = prg
		}
	}

	// Compile content hash (optional)
	if def.Observation.ContentHash != "" {
		cs.contentHash, err = compile(strings.TrimSpace(def.Observation.ContentHash))
		if err != nil {
			return nil, fmt.Errorf("compile observation.content_hash: %w", err)
		}
	}

	// Compile field mappings
	if len(def.FieldMappings) > 0 {
		for i, mapping := range def.FieldMappings {
			prg, err := compile(strings.TrimSpace(mapping.Source))
			if err != nil {
				return nil, fmt.Errorf("compile field_mappings[%d].source: %w", i, err)
			}
			cs.fieldMappings = append(cs.fieldMappings, CompiledFieldMapping{
				Program: prg,
				Target:  mapping.Target,
				Type:    mapping.Type,
			})
		}
	}

	// Compile pagination stop_when expression (optional).
	// Uses a separate CEL environment with `records` (list) variable instead of `record` (map).
	if def.Transport.Pagination != nil && def.Transport.Pagination.StopWhen != "" {
		cs.stopWhen, err = l.compiler.CompileStopWhen(strings.TrimSpace(def.Transport.Pagination.StopWhen))
		if err != nil {
			return nil, fmt.Errorf("compile pagination.stop_when: %w", err)
		}
	}

	// Compile media_actions path expressions (optional). Each uses a dedicated
	// environment whose only variable is the entity's merged metadata map.
	if len(def.MediaActions) > 0 {
		cs.mediaActions = make(map[string]cel.Program, len(def.MediaActions))
		for _, a := range def.MediaActions {
			prg, compileErr := l.compiler.CompileMediaActionPath(strings.TrimSpace(a.Path))
			if compileErr != nil {
				return nil, fmt.Errorf("compile media_actions[%s].path: %w", a.Name, compileErr)
			}
			cs.mediaActions[a.Name] = prg
		}
	}

	// Compile entity cache key (optional, v2)
	if def.EntityCache != nil && def.EntityCache.Key != "" {
		cs.entityCacheKey, err = compile(strings.TrimSpace(def.EntityCache.Key))
		if err != nil {
			return nil, fmt.Errorf("compile entity_cache.key: %w", err)
		}
	}

	return cs, nil
}

// validateTransportSpec performs conditional validation based on transport type.
func (l *Loader) validateTransportSpec(def *SourceDefinition, path string) error {
	ts := &def.Transport

	// URL required for specific transports
	switch ts.Type {
	case "http_poll", "websocket", "sse":
		if ts.URL == "" {
			return fmt.Errorf("validate %q: transport.url is required for type %q", path, ts.Type)
		}
	}

	// Interval required for pull-based transports
	switch ts.Type {
	case "http_poll", "ftp_sftp", "s3_poll":
		if ts.Interval.Duration <= 0 {
			return fmt.Errorf("validate %q: transport.interval is required for type %q", path, ts.Type)
		}
	}

	// Timeout is required for all transports
	if ts.Timeout.Duration <= 0 {
		return fmt.Errorf("validate %q: transport.timeout is required", path)
	}

	// Validate transport-specific sub-spec is present
	switch ts.Type {
	case "mqtt":
		if ts.MQTT == nil {
			return fmt.Errorf("validate %q: transport.mqtt config is required for type %q", path, ts.Type)
		}
	case "webhook":
		if ts.Webhook == nil {
			return fmt.Errorf("validate %q: transport.webhook config is required for type %q", path, ts.Type)
		}
	case "grpc_stream":
		if ts.GRPCStream == nil {
			return fmt.Errorf("validate %q: transport.grpc_stream config is required for type %q", path, ts.Type)
		}
	case "kafka":
		if ts.Kafka == nil {
			return fmt.Errorf("validate %q: transport.kafka config is required for type %q", path, ts.Type)
		}
	case "amqp":
		if ts.AMQP == nil {
			return fmt.Errorf("validate %q: transport.amqp config is required for type %q", path, ts.Type)
		}
	case "tcp_udp":
		if ts.TCPUDP == nil {
			return fmt.Errorf("validate %q: transport.tcp_udp config is required for type %q", path, ts.Type)
		}
	case "ftp_sftp":
		if ts.FTPSFTP == nil {
			return fmt.Errorf("validate %q: transport.ftp_sftp config is required for type %q", path, ts.Type)
		}
	case "s3_poll":
		if ts.S3Poll == nil {
			return fmt.Errorf("validate %q: transport.s3_poll config is required for type %q", path, ts.Type)
		}
	case "nats":
		if ts.NATS == nil {
			return fmt.Errorf("validate %q: transport.nats config is required for type %q", path, ts.Type)
		}
	}

	// Gate streaming transports behind schema_version >= 2
	streamingTypes := map[string]bool{
		"websocket": true, "sse": true, "mqtt": true, "webhook": true,
		"grpc_stream": true, "kafka": true, "amqp": true,
		"tcp_udp": true, "ftp_sftp": true, "s3_poll": true, "nats": true,
	}
	if streamingTypes[ts.Type] && def.SchemaVersion < 2 {
		return fmt.Errorf("validate %q: transport type %q requires schema_version >= 2", path, ts.Type)
	}

	return nil
}
