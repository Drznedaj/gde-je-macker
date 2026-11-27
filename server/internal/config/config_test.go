package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" || c.DatabaseURL != defaultDatabaseURL || c.PhotoDir != "./photos" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{"PORT": "9000", "DATABASE_URL": "postgres://x", "PHOTO_DIR": "/data"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9000" || c.DatabaseURL != "postgres://x" || c.PhotoDir != "/data" {
		t.Fatalf("unexpected: %+v", c)
	}
}

func TestInvalidPort(t *testing.T) {
	for _, p := range []string{"abc", "0", "70000"} {
		if _, err := Load(env(map[string]string{"PORT": p})); err == nil {
			t.Fatalf("expected error for PORT=%q", p)
		}
	}
}

func TestProductionRequiresDatabaseURL(t *testing.T) {
	if _, err := Load(env(map[string]string{"ENV": "production"})); err == nil {
		t.Fatal("expected error when DATABASE_URL missing in production")
	}
	if _, err := Load(env(map[string]string{"ENV": "production", "DATABASE_URL": ""})); err == nil {
		t.Fatal("expected error when DATABASE_URL empty in production")
	}
	c, err := Load(env(map[string]string{"ENV": "production", "DATABASE_URL": "postgres://x"}))
	if err != nil || c.DatabaseURL != "postgres://x" {
		t.Fatalf("unexpected: %+v %v", c, err)
	}
	if _, err := Load(env(map[string]string{"ENV": "development"})); err != nil {
		t.Fatalf("dev default should apply: %v", err)
	}
}

func TestTimeouts(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.ReadHeaderTimeout != 5*time.Second || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
		t.Fatalf("bad default timeouts: %+v", c)
	}
	c, err = Load(env(map[string]string{"WRITE_TIMEOUT": "45s"}))
	if err != nil || c.WriteTimeout != 45*time.Second {
		t.Fatalf("override failed: %+v %v", c, err)
	}
	for _, v := range []string{"abc", "0", "-1s"} {
		if _, err := Load(env(map[string]string{"READ_TIMEOUT": v})); err == nil {
			t.Fatalf("expected error for %q", v)
		}
	}
}
