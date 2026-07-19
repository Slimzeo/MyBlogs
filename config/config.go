package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
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
	CookieSecure  bool   // mark auth cookies Secure when TLS terminates at the app or proxy

	// Behaviour
	UploadDir            string // where uploaded attachments are written
	HitFlushEvery        int    // flush accumulated article hits after this many views
	RateLimitRPS         int    // per-client request/second limit (0 = disabled)
	RateLimitBurst       int
	AdminUsername        string // bootstrap username used only when the user table is empty
	AdminEmail           string // bootstrap email used only when the user table is empty
	AdminInitialPassword string // bootstrap password used only when the user table is empty
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

		SessionSecret: env("SESSION_SECRET", ""),
		CookieSecure:  envBool("COOKIE_SECURE", false),

		UploadDir:            env("UPLOAD_DIR", "data/upload"),
		HitFlushEvery:        envInt("HIT_FLUSH_EVERY", 100),
		RateLimitRPS:         envInt("RATE_LIMIT_RPS", 200),
		RateLimitBurst:       envInt("RATE_LIMIT_BURST", 400),
		AdminUsername:        env("ADMIN_USERNAME", ""),
		AdminEmail:           env("ADMIN_EMAIL", ""),
		AdminInitialPassword: env("ADMIN_INITIAL_PASSWORD", ""),
	}

	if c.DBDSN == "" {
		switch c.DBDriver {
		case "mysql":
			// The MySQL DSN contains credentials and must be supplied by the
			// deployment environment rather than committed to the repository.
			c.DBDSN = ""
		default: // sqlite
			c.DBDriver = "sqlite"
			// WAL + busy timeout so concurrent readers/writers don't block hard.
			c.DBDSN = "data/blog.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
		}
	}
	return c
}

// Validate rejects unsafe or incomplete deployment configuration before any
// database connection or HTTP listener is created.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.SessionSecret) == "" {
		return errors.New("SESSION_SECRET must be set")
	}
	if len([]byte(c.SessionSecret)) < 32 {
		return errors.New("SESSION_SECRET must contain at least 32 bytes")
	}
	if c.DBDriver == "mysql" && strings.TrimSpace(c.DBDSN) == "" {
		return errors.New("DB_DSN must be set when DB_DRIVER=mysql")
	}
	bootstrapConfigured := c.AdminUsername != "" || c.AdminEmail != "" || c.AdminInitialPassword != ""
	if bootstrapConfigured {
		if strings.TrimSpace(c.AdminUsername) == "" ||
			strings.TrimSpace(c.AdminEmail) == "" ||
			c.AdminInitialPassword == "" {
			return errors.New("ADMIN_USERNAME, ADMIN_EMAIL and ADMIN_INITIAL_PASSWORD must be provided together")
		}
		if len([]rune(c.AdminInitialPassword)) < 6 {
			return errors.New("ADMIN_INITIAL_PASSWORD must contain at least 6 characters")
		}
	}
	return nil
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

func envBool(key string, def bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return def
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return def
	}
	return parsed
}
