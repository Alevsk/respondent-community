package config_test

// Tests for internal/app/feeder/config, covering:
//   - LoadFromViper() with a fully populated valid config
//   - LoadFromViper() with default (empty) viper state
//   - LoadFromViper() with type-incompatible values that force unmarshal errors
//   - Individual section field values after a successful load
//   - Config struct zero-values as a guard against accidental field removal

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/app/feeder/config"
	sharedconfig "github.com/Alevsk/respondent/internal/config"
)

// resetViper clears all viper state between tests.
func resetViper() {
	viper.Reset()
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_Valid
// ---------------------------------------------------------------------------

// TestLoadFromViper_Valid verifies that LoadFromViper correctly populates all
// config sections when valid values are set in viper.
func TestLoadFromViper_Valid(t *testing.T) {
	resetViper()
	defer resetViper()

	// Database
	viper.Set("database.host", "db.example.com")
	viper.Set("database.port", 5432)
	viper.Set("database.name", "respondent_db")
	viper.Set("database.user", "appuser")
	viper.Set("database.password", "s3cr3t")
	viper.Set("database.ssl_mode", "require")
	viper.Set("database.max_conns", 20)
	viper.Set("database.min_conns", 4)
	viper.Set("database.max_conn_lifetime", "2h")

	// Valkey
	viper.Set("valkey.addr", "valkey:6379")
	viper.Set("valkey.password", "vk_pass")
	viper.Set("valkey.db", 3)
	viper.Set("valkey.pool_size", 15)
	viper.Set("valkey.min_idle_conns", 5)

	// Ingest
	viper.Set("ingest.sources_dir", "/etc/respondent/sources.d")
	viper.Set("ingest.dev_mode", true)

	// Feeder
	viper.Set("feeder.batch_size", 500)
	viper.Set("feeder.worker_count", 8)
	viper.Set("feeder.retry_limit", 3)
	viper.Set("feeder.retry_backoff", "5s")

	// Logging
	viper.Set("logging.level", "debug")
	viper.Set("logging.format", "json")

	cfg, err := config.LoadFromViper()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Database section
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "respondent_db", cfg.Database.Name)
	assert.Equal(t, "appuser", cfg.Database.User)
	assert.Equal(t, "s3cr3t", cfg.Database.Password)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, 20, cfg.Database.MaxConns)
	assert.Equal(t, 4, cfg.Database.MinConns)
	assert.Equal(t, 2*time.Hour, cfg.Database.MaxConnLifetime)

	// Valkey section
	assert.Equal(t, "valkey:6379", cfg.Valkey.Addr)
	assert.Equal(t, "vk_pass", cfg.Valkey.Password)
	assert.Equal(t, 3, cfg.Valkey.DB)
	assert.Equal(t, 15, cfg.Valkey.PoolSize)
	assert.Equal(t, 5, cfg.Valkey.MinIdleConn)

	// Ingest section
	assert.Equal(t, "/etc/respondent/sources.d", cfg.Ingest.SourcesDir)
	assert.True(t, cfg.Ingest.DevMode)

	// Feeder section
	assert.Equal(t, 500, cfg.Feeder.BatchSize)
	assert.Equal(t, 8, cfg.Feeder.WorkerCount)
	assert.Equal(t, 3, cfg.Feeder.RetryLimit)
	assert.Equal(t, 5*time.Second, cfg.Feeder.RetryBackoff)

	// Logging section
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_Defaults
// ---------------------------------------------------------------------------

// TestLoadFromViper_Defaults verifies that LoadFromViper succeeds with an empty
// viper state (all zero/empty values) and returns a non-nil config.
func TestLoadFromViper_Defaults(t *testing.T) {
	resetViper()
	defer resetViper()

	cfg, err := config.LoadFromViper()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// With no values set every field is at its zero value.
	assert.Empty(t, cfg.Database.Host)
	assert.Empty(t, cfg.Valkey.Addr)
	assert.Empty(t, cfg.Ingest.SourcesDir)
	assert.False(t, cfg.Ingest.DevMode)
	assert.Zero(t, cfg.Feeder.BatchSize)
	assert.Zero(t, cfg.Feeder.WorkerCount)
	assert.Empty(t, cfg.Logging.Level)
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_DatabaseSection
// ---------------------------------------------------------------------------

// TestLoadFromViper_DatabaseSection exercises all DatabaseConfig fields individually.
func TestLoadFromViper_DatabaseSection(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		validate func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "host and port populated",
			setup: func() {
				viper.Set("database.host", "pg-host")
				viper.Set("database.port", 5433)
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "pg-host", cfg.Database.Host)
				assert.Equal(t, 5433, cfg.Database.Port)
			},
		},
		{
			name: "ssl_mode set to require",
			setup: func() {
				viper.Set("database.ssl_mode", "require")
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "require", cfg.Database.SSLMode)
			},
		},
		{
			name: "connection pool limits",
			setup: func() {
				viper.Set("database.max_conns", 50)
				viper.Set("database.min_conns", 5)
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 50, cfg.Database.MaxConns)
				assert.Equal(t, 5, cfg.Database.MinConns)
			},
		},
		{
			name: "max_conn_lifetime as duration string",
			setup: func() {
				viper.Set("database.max_conn_lifetime", "30m")
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 30*time.Minute, cfg.Database.MaxConnLifetime)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()
			tt.setup()

			cfg, err := config.LoadFromViper()
			require.NoError(t, err)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_ValkeySection
// ---------------------------------------------------------------------------

// TestLoadFromViper_ValkeySection verifies individual ValkeyConfig fields.
func TestLoadFromViper_ValkeySection(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		validate func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "addr and password",
			setup: func() {
				viper.Set("valkey.addr", "cache:6379")
				viper.Set("valkey.password", "pass123")
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "cache:6379", cfg.Valkey.Addr)
				assert.Equal(t, "pass123", cfg.Valkey.Password)
			},
		},
		{
			name: "db index and pool settings",
			setup: func() {
				viper.Set("valkey.db", 2)
				viper.Set("valkey.pool_size", 25)
				viper.Set("valkey.min_idle_conns", 3)
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 2, cfg.Valkey.DB)
				assert.Equal(t, 25, cfg.Valkey.PoolSize)
				assert.Equal(t, 3, cfg.Valkey.MinIdleConn)
			},
		},
		{
			name: "zero db index (default db)",
			setup: func() {
				viper.Set("valkey.addr", "localhost:6379")
				viper.Set("valkey.db", 0)
			},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, 0, cfg.Valkey.DB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()
			tt.setup()

			cfg, err := config.LoadFromViper()
			require.NoError(t, err)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_IngestSection
// ---------------------------------------------------------------------------

// TestLoadFromViper_IngestSection verifies individual IngestConfig fields.
func TestLoadFromViper_IngestSection(t *testing.T) {
	tests := []struct {
		name       string
		sourcesDir string
		devMode    bool
	}{
		{
			name:       "production config",
			sourcesDir: "/etc/respondent/sources.d",
			devMode:    false,
		},
		{
			name:       "dev config",
			sourcesDir: "./testdata/sources.d",
			devMode:    true,
		},
		{
			name:       "empty sources dir",
			sourcesDir: "",
			devMode:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			viper.Set("ingest.sources_dir", tt.sourcesDir)
			viper.Set("ingest.dev_mode", tt.devMode)

			cfg, err := config.LoadFromViper()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tt.sourcesDir, cfg.Ingest.SourcesDir)
			assert.Equal(t, tt.devMode, cfg.Ingest.DevMode)
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_FeederSection
// ---------------------------------------------------------------------------

// TestLoadFromViper_FeederSection verifies individual FeederConfig fields.
func TestLoadFromViper_FeederSection(t *testing.T) {
	tests := []struct {
		name         string
		batchSize    int
		workerCount  int
		retryLimit   int
		retryBackoff string
		wantBackoff  time.Duration
	}{
		{
			name:         "standard feeder settings",
			batchSize:    100,
			workerCount:  4,
			retryLimit:   3,
			retryBackoff: "1s",
			wantBackoff:  time.Second,
		},
		{
			name:         "high-throughput settings",
			batchSize:    1000,
			workerCount:  16,
			retryLimit:   5,
			retryBackoff: "500ms",
			wantBackoff:  500 * time.Millisecond,
		},
		{
			name:         "aggressive retry backoff",
			batchSize:    50,
			workerCount:  2,
			retryLimit:   10,
			retryBackoff: "2m",
			wantBackoff:  2 * time.Minute,
		},
		{
			name:         "minimal feeder",
			batchSize:    1,
			workerCount:  1,
			retryLimit:   1,
			retryBackoff: "100ms",
			wantBackoff:  100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			viper.Set("feeder.batch_size", tt.batchSize)
			viper.Set("feeder.worker_count", tt.workerCount)
			viper.Set("feeder.retry_limit", tt.retryLimit)
			viper.Set("feeder.retry_backoff", tt.retryBackoff)

			cfg, err := config.LoadFromViper()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tt.batchSize, cfg.Feeder.BatchSize)
			assert.Equal(t, tt.workerCount, cfg.Feeder.WorkerCount)
			assert.Equal(t, tt.retryLimit, cfg.Feeder.RetryLimit)
			assert.Equal(t, tt.wantBackoff, cfg.Feeder.RetryBackoff)
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_LoggingSection
// ---------------------------------------------------------------------------

// TestLoadFromViper_LoggingSection verifies LoggingConfig field combinations.
func TestLoadFromViper_LoggingSection(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		format     string
		wantLevel  string
		wantFormat string
	}{
		{
			name:       "json production logging",
			level:      "warn",
			format:     "json",
			wantLevel:  "warn",
			wantFormat: "json",
		},
		{
			name:       "console debug logging",
			level:      "debug",
			format:     "console",
			wantLevel:  "debug",
			wantFormat: "console",
		},
		{
			name:       "info level",
			level:      "info",
			format:     "json",
			wantLevel:  "info",
			wantFormat: "json",
		},
		{
			name:       "error level",
			level:      "error",
			format:     "console",
			wantLevel:  "error",
			wantFormat: "console",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			viper.Set("logging.level", tt.level)
			viper.Set("logging.format", tt.format)

			cfg, err := config.LoadFromViper()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, tt.wantLevel, cfg.Logging.Level)
			assert.Equal(t, tt.wantFormat, cfg.Logging.Format)
		})
	}
}

// ---------------------------------------------------------------------------
// TestConfig_Struct_ZeroValues
// ---------------------------------------------------------------------------

// TestConfig_Struct_ZeroValues guards against accidental removal of Config fields
// by asserting each section's zero-value type is correct.
func TestConfig_Struct_ZeroValues(t *testing.T) {
	var cfg config.Config

	// Ensure all sections are of the correct embedded struct type.
	assert.IsType(t, sharedconfig.DatabaseConfig{}, cfg.Database)
	assert.IsType(t, sharedconfig.ValkeyConfig{}, cfg.Valkey)
	assert.IsType(t, sharedconfig.IngestConfig{}, cfg.Ingest)
	assert.IsType(t, sharedconfig.FeederConfig{}, cfg.Feeder)
	assert.IsType(t, sharedconfig.LoggingConfig{}, cfg.Logging)

	// Zero values
	assert.Empty(t, cfg.Database.Host)
	assert.Zero(t, cfg.Database.Port)
	assert.Empty(t, cfg.Valkey.Addr)
	assert.Zero(t, cfg.Valkey.DB)
	assert.Empty(t, cfg.Ingest.SourcesDir)
	assert.False(t, cfg.Ingest.DevMode)
	assert.Zero(t, cfg.Feeder.BatchSize)
	assert.Zero(t, cfg.Feeder.WorkerCount)
	assert.Empty(t, cfg.Logging.Level)
	assert.Empty(t, cfg.Logging.Format)
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_DatabaseDSN
// ---------------------------------------------------------------------------

// TestLoadFromViper_DatabaseDSN verifies that the DatabaseConfig.GetDSN() method
// builds the expected connection string from values loaded via viper.
func TestLoadFromViper_DatabaseDSN(t *testing.T) {
	resetViper()
	defer resetViper()

	viper.Set("database.host", "pghost")
	viper.Set("database.port", 5432)
	viper.Set("database.name", "mydb")
	viper.Set("database.user", "myuser")
	viper.Set("database.password", "mypass")
	viper.Set("database.ssl_mode", "disable")

	cfg, err := config.LoadFromViper()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	dsn := cfg.Database.GetDSN()
	assert.Contains(t, dsn, "host=pghost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "dbname=mydb")
	assert.Contains(t, dsn, "user=myuser")
	assert.Contains(t, dsn, "password=mypass")
	assert.Contains(t, dsn, "sslmode=disable")
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_MultipleCallsIsolated
// ---------------------------------------------------------------------------

// TestLoadFromViper_MultipleCallsIsolated ensures that successive calls to
// LoadFromViper with different viper states return independent configs.
func TestLoadFromViper_MultipleCallsIsolated(t *testing.T) {
	// First call
	resetViper()
	viper.Set("logging.level", "info")
	viper.Set("feeder.batch_size", 100)

	cfg1, err := config.LoadFromViper()
	require.NoError(t, err)

	// Second call with different state
	resetViper()
	viper.Set("logging.level", "debug")
	viper.Set("feeder.batch_size", 200)

	cfg2, err := config.LoadFromViper()
	require.NoError(t, err)
	defer resetViper()

	assert.Equal(t, "info", cfg1.Logging.Level)
	assert.Equal(t, 100, cfg1.Feeder.BatchSize)

	assert.Equal(t, "debug", cfg2.Logging.Level)
	assert.Equal(t, 200, cfg2.Feeder.BatchSize)

	// The two configs must be independent.
	assert.NotEqual(t, cfg1.Logging.Level, cfg2.Logging.Level)
	assert.NotEqual(t, cfg1.Feeder.BatchSize, cfg2.Feeder.BatchSize)
}

// ---------------------------------------------------------------------------
// TestLoadFromViper_UnmarshalErrors
// ---------------------------------------------------------------------------

// badDuration is a helper that produces a map value that mapstructure cannot
// decode into a time.Duration field, reliably triggering an UnmarshalKey error.
func badDuration() map[string]interface{} {
	return map[string]interface{}{"not": "a-duration"}
}

// TestLoadFromViper_UnmarshalErrors verifies that LoadFromViper propagates
// errors returned by viper.Unmarshal for config sections that still use it.
// Currently only the "ai" section is unmarshalled via viper.Sub().Unmarshal();
// all other sections use direct viper getters and cannot trigger unmarshal errors.
func TestLoadFromViper_UnmarshalErrors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		errSubstr string
	}{
		{
			// ai section: sub.Unmarshal receives a bad duration, triggering decode error.
			name: "ai unmarshal error",
			setup: func() {
				viper.Set("ai.search.query_timeout", badDuration())
			},
			errSubstr: "failed to unmarshal ai config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			tt.setup()

			cfg, err := config.LoadFromViper()
			require.Error(t, err, "expected an error from LoadFromViper")
			assert.Nil(t, cfg, "cfg must be nil when an error is returned")
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}
