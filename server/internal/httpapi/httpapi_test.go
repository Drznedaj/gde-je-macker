package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	NewRouter(p, quiet, nil).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
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

func TestAccessLogUsesRoutePatternNotPath(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts/{id}/manage/{token}", func(w http.ResponseWriter, r *http.Request) {})
	h := logging(log, mux)
	const secret = "s3cr3t-T0KEN-abc123"
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/posts/42/manage/"+secret+"?t="+secret, nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nope/"+secret, nil))
	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("log leaked token: %s", out)
	}
	if !strings.Contains(out, "/posts/{id}/manage/{token}") || !strings.Contains(out, "unmatched") {
		t.Fatalf("route pattern missing: %s", out)
	}
}

func TestPingErrorLogIsSanitized(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	dsn := "postgres://macker:hunter2@db.internal.example:5432/macker"
	pingErr := errors.New("failed to connect to " + dsn + ": password hunter2 host db.internal.example")
	rec := httptest.NewRecorder()
	NewRouter(fakeDB{err: pingErr}, log, nil).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	out := buf.String()
	for _, bad := range []string{"hunter2", "db.internal.example", "postgres://", "macker:"} {
		if strings.Contains(out, bad) {
			t.Fatalf("log leaked %q: %s", bad, out)
		}
	}
	if !strings.Contains(out, "db ping failed") {
		t.Fatalf("ping failure not logged: %s", out)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatal("response leaked credentials")
	}
}

func status(h http.Handler, path string) (int, string, string) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()
}

func TestVersionedRouting(t *testing.T) {
	h := NewRouter(fakeDB{}, quiet, nil)
	if c, _, _ := status(h, "/healthz"); c != http.StatusOK {
		t.Fatalf("/healthz got %d", c)
	}
	for _, p := range []string{"/api/v1/healthz", "/posts"} {
		if c, _, _ := status(h, p); c != http.StatusNotFound {
			t.Fatalf("%s got %d", p, c)
		}
	}
	c, ct, body := status(h, "/api/v1/nope")
	if c != http.StatusNotFound || !strings.HasPrefix(ct, "application/json") || !strings.Contains(body, "not_found") {
		t.Fatalf("got %d %q %q", c, ct, body)
	}
}

func TestV1HandlerMountedAndLoggedByPattern(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	h := newRouter(fakeDB{}, log, func(v1 *http.ServeMux) {
		v1.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {})
	})
	const secret = "s3cr3t-T0KEN"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/posts/"+secret+"?q="+secret, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	out := buf.String()
	if strings.Contains(out, secret) || !strings.Contains(out, "/api/v1/posts/{id}") {
		t.Fatalf("bad log: %s", out)
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
