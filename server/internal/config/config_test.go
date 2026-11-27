package config

import "testing"

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
