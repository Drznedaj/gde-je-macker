// Package config loads server settings from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings. Defaults match docker-compose.yml.
type Config struct {
	DatabaseURL string
	Port        string
	PhotoDir    string
	Env         string

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

const (
	defaultDatabaseURL = "postgres://macker:macker@127.0.0.1:5432/macker?sslmode=disable"
	defaultPort        = "8080"
	defaultPhotoDir    = "./photos"
)

// Load reads configuration using the given lookup function (os.LookupEnv in production).
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return def
	}
	env := get("ENV", "development")
	dbURL := get("DATABASE_URL", "")
	if dbURL == "" {
		if env == "production" {
			return Config{}, errors.New("DATABASE_URL is required when ENV=production")
		}
		dbURL = defaultDatabaseURL
	}
	c := Config{
		Env:         env,
		DatabaseURL: dbURL,
		Port:        get("PORT", defaultPort),
		PhotoDir:    get("PHOTO_DIR", defaultPhotoDir),
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q", c.Port)
	}
	durs := []struct {
		key string
		def time.Duration
		dst *time.Duration
	}{
		{"READ_HEADER_TIMEOUT", 5 * time.Second, &c.ReadHeaderTimeout},
		{"READ_TIMEOUT", 15 * time.Second, &c.ReadTimeout},
		{"WRITE_TIMEOUT", 30 * time.Second, &c.WriteTimeout},
		{"IDLE_TIMEOUT", 60 * time.Second, &c.IdleTimeout},
		{"SHUTDOWN_TIMEOUT", 10 * time.Second, &c.ShutdownTimeout},
	}
	for _, d := range durs {
		v := get(d.key, "")
		if v == "" {
			*d.dst = d.def
			continue
		}
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("invalid %s %q", d.key, v)
		}
		*d.dst = parsed
	}
	return c, nil
}

// FromEnv loads configuration from the process environment.
func FromEnv() (Config, error) { return Load(os.LookupEnv) }
