// Package config loads server settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime settings. Defaults match docker-compose.yml.
type Config struct {
	DatabaseURL string
	Port        string
	PhotoDir    string
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
	c := Config{
		DatabaseURL: get("DATABASE_URL", defaultDatabaseURL),
		Port:        get("PORT", defaultPort),
		PhotoDir:    get("PHOTO_DIR", defaultPhotoDir),
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q", c.Port)
	}
	return c, nil
}

// FromEnv loads configuration from the process environment.
func FromEnv() (Config, error) { return Load(os.LookupEnv) }
