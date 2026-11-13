package config

import "time"

// Fixed runtime settings are maintained here rather than in external configuration.
const (
	HTTPReadHeaderTimeout = 5 * time.Second
	HTTPReadTimeout       = 30 * time.Second
	HTTPRequestTimeout    = 30 * time.Second
	HTTPWriteTimeout      = 35 * time.Second
	HTTPIdleTimeout       = 60 * time.Second
	HTTPShutdownTimeout   = 35 * time.Second

	DatabasePingTimeout = 5 * time.Second
	HealthCheckTimeout  = 2 * time.Second
	SQLExecutionTimeout = 30 * time.Second

	LogProgressInterval = 30 * time.Second
	IdempotencyCacheTTL = 30 * time.Minute
)
