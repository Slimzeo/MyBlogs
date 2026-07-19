package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration. Values come from environment
// variables so the app runs out-of-the-box with sane defaults (SQLite),
// and can be pointed at MySQL for production by setting DB_DRIVER=mysql.
type Config struct {
	// Server
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// Database
	DBDriver string // "sqlite" (default) or "mysql"
	DBDSN    string // full DSN; if empty a default is derived from driver

	// Connection pool (tuned for high concurrency)
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Security
	SessionSecret string // cookie session signing key

	// Behaviour
	UploadDir      string // where uploaded attachments are written
	HitFlushEvery  int    // flush accumulated article hits after this many views
	RateLimitRPS   int    // per-client request/second limit (0 = disabled)
	RateLimitBurst int
}

// Load reads configuration from the environment applying defaults.
func Load() *Config {
	c := &Config{
		Port:            env("PORT", "8081"),
		ReadTimeout:     time.Duration(envInt("READ_TIMEOUT_SEC", 15)) * time.Second,
		WriteTimeout:    time.Duration(envInt("WRITE_TIMEOUT_SEC", 30)) * time.Second,
		ShutdownTimeout: time.Duration(envInt("SHUTDOWN_TIMEOUT_SEC", 10)) * time.Second,

		DBDriver: env("DB_DRIVER", "sqlite"),
		DBDSN:    env("DB_DSN", ""),

		DBMaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 100),
		DBMaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 20),
		DBConnMaxLifetime: time.Duration(envInt("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,

		SessionSecret: env("SESSION_SECRET", "my-blog-please-change-this-secret"),

		UploadDir:      env("UPLOAD_DIR", "data/upload"),
		HitFlushEvery:  envInt("HIT_FLUSH_EVERY", 100),
		RateLimitRPS:   envInt("RATE_LIMIT_RPS", 200),
		RateLimitBurst: envInt("RATE_LIMIT_BURST", 400),
	}

	if c.DBDSN == "" {
		switch c.DBDriver {
		case "mysql":
			// user:pass@tcp(host:port)/dbname?...
			c.DBDSN = "root:root@tcp(127.0.0.1:3306)/tale?charset=utf8mb4&parseTime=True&loc=Local"
		default: // sqlite
			c.DBDriver = "sqlite"
			// WAL + busy timeout so concurrent readers/writers don't block hard.
			c.DBDSN = "data/blog.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
		}
	}
	return c
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
