package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func get(t *testing.T, p Pinger) int {
	t.Helper()
	rec := httptest.NewRecorder()
	NewRouter(p, quiet).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	return rec.Code
}

func TestHealthzDown(t *testing.T) {
	if c := get(t, fakeDB{err: errors.New("boom")}); c != http.StatusServiceUnavailable {
		t.Fatalf("got %d", c)
	}
}

func TestHealthzOKWithFake(t *testing.T) {
	if c := get(t, fakeDB{}); c != http.StatusOK {
		t.Fatalf("got %d", c)
	}
}

// Integration test: skipped when no Postgres is reachable.
func TestHealthzIntegration(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://macker:macker@127.0.0.1:5432/macker?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skipf("no database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database unreachable: %v", err)
	}
	if c := get(t, pool); c != http.StatusOK {
		t.Fatalf("got %d", c)
	}
}
