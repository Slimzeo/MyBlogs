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
// and can be pointed at MySQL for production by setting BLOG_DB_DRIVER=mysql.
type Config struct {
	// Server
	Port            string
	GinMode         string
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
		Port:            env("BLOG_PORT", "8081"),
		GinMode:         env("BLOG_GIN_MODE", "release"),
		ReadTimeout:     time.Duration(envInt("BLOG_READ_TIMEOUT_SEC", 15)) * time.Second,
		WriteTimeout:    time.Duration(envInt("BLOG_WRITE_TIMEOUT_SEC", 30)) * time.Second,
		ShutdownTimeout: time.Duration(envInt("BLOG_SHUTDOWN_TIMEOUT_SEC", 10)) * time.Second,

		DBDriver: env("BLOG_DB_DRIVER", "sqlite"),
		DBDSN:    env("BLOG_DB_DSN", ""),

		DBMaxOpenConns:    envInt("BLOG_DB_MAX_OPEN_CONNS", 100),
		DBMaxIdleConns:    envInt("BLOG_DB_MAX_IDLE_CONNS", 20),
		DBConnMaxLifetime: time.Duration(envInt("BLOG_DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,

		SessionSecret: env("BLOG_SESSION_SECRET", ""),
		CookieSecure:  envBool("BLOG_COOKIE_SECURE", false),

		UploadDir:            env("BLOG_UPLOAD_DIR", "data/upload"),
		HitFlushEvery:        envInt("BLOG_HIT_FLUSH_EVERY", 100),
		RateLimitRPS:         envInt("BLOG_RATE_LIMIT_RPS", 200),
		RateLimitBurst:       envInt("BLOG_RATE_LIMIT_BURST", 400),
		AdminUsername:        env("BLOG_ADMIN_USERNAME", ""),
		AdminEmail:           env("BLOG_ADMIN_EMAIL", ""),
		AdminInitialPassword: env("BLOG_ADMIN_INITIAL_PASSWORD", ""),
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
		return errors.New("BLOG_SESSION_SECRET must be set")
	}
	if len([]byte(c.SessionSecret)) < 32 {
		return errors.New("BLOG_SESSION_SECRET must contain at least 32 bytes")
	}
	if c.DBDriver == "mysql" && strings.TrimSpace(c.DBDSN) == "" {
		return errors.New("BLOG_DB_DSN must be set when BLOG_DB_DRIVER=mysql")
	}
	bootstrapConfigured := c.AdminUsername != "" || c.AdminEmail != "" || c.AdminInitialPassword != ""
	if bootstrapConfigured {
		if strings.TrimSpace(c.AdminUsername) == "" ||
			strings.TrimSpace(c.AdminEmail) == "" ||
			c.AdminInitialPassword == "" {
			return errors.New("BLOG_ADMIN_USERNAME, BLOG_ADMIN_EMAIL and BLOG_ADMIN_INITIAL_PASSWORD must be provided together")
		}
		if len([]rune(c.AdminInitialPassword)) < 6 {
			return errors.New("BLOG_ADMIN_INITIAL_PASSWORD must contain at least 6 characters")
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
