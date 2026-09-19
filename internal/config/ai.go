package config

import "time"

// AIConfig holds AI feature configuration for respondent.yaml.
type AIConfig struct {
	Enabled             bool           `yaml:"enabled" mapstructure:"enabled"`
	Workers             WorkersConfig  `yaml:"workers" mapstructure:"workers"`
	Search              SearchConfig   `yaml:"search" mapstructure:"search"`
	NATS                AINATSConfig   `yaml:"nats" mapstructure:"nats"`
	Insights            InsightsConfig `yaml:"insights" mapstructure:"insights"`
	Budget              BudgetConfig   `yaml:"budget" mapstructure:"budget"`
	AnalysisDir         string         `yaml:"analysis_dir" mapstructure:"analysis_dir"`
	TasksDir            string         `yaml:"tasks_dir" mapstructure:"tasks_dir"`
	TargetsDir          string         `yaml:"targets_dir" mapstructure:"targets_dir"`
	NotificationSubject string         `yaml:"notification_subject" mapstructure:"notification_subject"`
}

// WorkersConfig defines concurrency and queue settings for AI workers.
type WorkersConfig struct {
	EnrichmentConcurrency int `yaml:"enrichment_concurrency" mapstructure:"enrichment_concurrency"`
	AnalysisConcurrency   int `yaml:"analysis_concurrency" mapstructure:"analysis_concurrency"`
	RateLimit             int `yaml:"rate_limit" mapstructure:"rate_limit"`
	MaxQueueSize          int `yaml:"max_queue_size" mapstructure:"max_queue_size"`
	DLQMaxRetries         int `yaml:"dlq_max_retries" mapstructure:"dlq_max_retries"`
}

// SearchConfig defines AI-powered search settings.
type SearchConfig struct {
	Enabled      bool          `yaml:"enabled" mapstructure:"enabled"`
	MaxResults   int           `yaml:"max_results" mapstructure:"max_results"`
	QueryTimeout time.Duration `yaml:"query_timeout" mapstructure:"query_timeout"`
	CacheTTL     time.Duration `yaml:"cache_ttl" mapstructure:"cache_ttl"`
}

// AINATSConfig defines NATS settings specific to AI message processing.
type AINATSConfig struct {
	URL                       string               `yaml:"url" mapstructure:"url"`
	StreamName                string               `yaml:"stream_name" mapstructure:"stream_name"`
	EnrichmentSubject         string               `yaml:"enrichment_subject" mapstructure:"enrichment_subject"`
	AnalysisSubject           string               `yaml:"analysis_subject" mapstructure:"analysis_subject"`
	ConsumerGroup             string               `yaml:"consumer_group" mapstructure:"consumer_group"`
	MaxDeliver                int                  `yaml:"max_deliver" mapstructure:"max_deliver"`
	AckWait                   time.Duration        `yaml:"ack_wait" mapstructure:"ack_wait"`
	StreamMaxMsgs             int64                `yaml:"stream_max_msgs" mapstructure:"stream_max_msgs"`
	StreamMaxBytes            int64                `yaml:"stream_max_bytes" mapstructure:"stream_max_bytes"`
	StreamMaxAge              time.Duration        `yaml:"stream_max_age" mapstructure:"stream_max_age"`
	CircuitBreaker            CircuitBreakerConfig `yaml:"circuit_breaker" mapstructure:"circuit_breaker"`
	TasksStreamName           string               `yaml:"tasks_stream_name" mapstructure:"tasks_stream_name"`
	EnrichmentCompleteSubject string               `yaml:"enrichment_complete_subject" mapstructure:"enrichment_complete_subject"`
	TasksCircuitBreaker       CircuitBreakerConfig `yaml:"tasks_circuit_breaker" mapstructure:"tasks_circuit_breaker"`
}

// CircuitBreakerConfig defines settings for the enrichment publisher circuit breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold" mapstructure:"failure_threshold"`
	InitialCooldown  time.Duration `yaml:"initial_cooldown" mapstructure:"initial_cooldown"`
	MaxCooldown      time.Duration `yaml:"max_cooldown" mapstructure:"max_cooldown"`
}

// InsightsConfig defines storage and delivery settings for AI-generated insights.
type InsightsConfig struct {
	Retention     time.Duration `yaml:"retention" mapstructure:"retention"`
	MaxPerEntity  int           `yaml:"max_per_entity" mapstructure:"max_per_entity"`
	WebSocketPush bool          `yaml:"websocket_push" mapstructure:"websocket_push"`
}

// BudgetConfig defines token budget limits and alert thresholds.
type BudgetConfig struct {
	DailyTokenLimit     int     `yaml:"daily_token_limit" mapstructure:"daily_token_limit"`
	AlertThreshold      float64 `yaml:"alert_threshold" mapstructure:"alert_threshold"`
	EnrichmentMaxTokens int     `yaml:"enrichment_max_tokens" mapstructure:"enrichment_max_tokens"`
	SearchMaxTokens     int     `yaml:"search_max_tokens" mapstructure:"search_max_tokens"`
	AnalysisMaxTokens   int     `yaml:"analysis_max_tokens" mapstructure:"analysis_max_tokens"`
}

// DefaultAIConfig returns sensible defaults for AI configuration.
func DefaultAIConfig() AIConfig {
	return AIConfig{
		Enabled: false,
		Workers: WorkersConfig{
			EnrichmentConcurrency: 4,
			AnalysisConcurrency:   2,
			RateLimit:             10,
			MaxQueueSize:          10000,
			DLQMaxRetries:         3,
		},
		Search: SearchConfig{
			Enabled:      true,
			MaxResults:   1000,
			QueryTimeout: 5 * time.Second,
			CacheTTL:     10 * time.Minute,
		},
		NATS: AINATSConfig{
			URL:               "nats://localhost:4222",
			StreamName:        "RESPONDENT_AI",
			EnrichmentSubject: "respondent.ai.enrich",
			AnalysisSubject:   "respondent.ai.analysis",
			ConsumerGroup:     "ai-workers",
			MaxDeliver:        3,
			AckWait:           60 * time.Second,
			StreamMaxMsgs:     10000,
			StreamMaxBytes:    52428800, // 50 MB
			StreamMaxAge:      24 * time.Hour,
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				InitialCooldown:  5 * time.Minute,
				MaxCooldown:      1 * time.Hour,
			},
			TasksStreamName:           "RESPONDENT_TASKS",
			EnrichmentCompleteSubject: "respondent.ai.enrichment.complete",
			TasksCircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				InitialCooldown:  5 * time.Minute,
				MaxCooldown:      1 * time.Hour,
			},
		},
		Insights: InsightsConfig{
			Retention:     168 * time.Hour,
			MaxPerEntity:  100,
			WebSocketPush: true,
		},
		Budget: BudgetConfig{
			DailyTokenLimit:     1000000,
			AlertThreshold:      0.8,
			EnrichmentMaxTokens: 500,
			SearchMaxTokens:     1000,
			AnalysisMaxTokens:   2000,
		},
		NotificationSubject: "respondent.notifications",
		AnalysisDir:         "analysis.d/",
		TasksDir:            "tasks.d/",
		TargetsDir:          "targets.d/",
	}
}
