// Package httpapi holds the HTTP router and middleware.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the HTTP handler. Requests are logged without bodies,
// query strings, raw paths or client addresses.
//
// /healthz lives on the root mux, unversioned. Everything else is served by a
// versioned sub-mux mounted at /api/v1/; handlers registered on it use paths
// relative to that prefix (for example "GET /posts").
func NewRouter(db Pinger, log *slog.Logger) http.Handler {
	return newRouter(db, log, nil)
}

// newRouter is NewRouter with a hook to register handlers on the v1 mux.
func newRouter(db Pinger, log *slog.Logger, mountV1 func(v1 *http.ServeMux)) http.Handler {
	v1 := http.NewServeMux()
	if mountV1 != nil {
		mountV1(v1)
	}
	v1.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(db, log))
	mux.Handle("/api/v1/", versioned(v1))
	return logging(log, mux)
}

// versioned strips the /api/v1 prefix and serves the sub-mux. The sub-mux
// matches on a copy of the request, so the matched pattern is reported back
// through the logging holder in the context.
func versioned(v1 *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := new(http.Request)
		*r2 = *r
		u := *r.URL
		u.Path = strings.TrimPrefix(r.URL.Path, "/api/v1")
		u.RawPath = ""
		r2.URL = &u
		v1.ServeHTTP(w, r2)
		if h, ok := r.Context().Value(routeKey{}).(*string); ok && r2.Pattern != "" {
			*h = prefixPattern(r2.Pattern)
		}
	})
}

// prefixPattern turns "GET /posts" into "GET /api/v1/posts".
func prefixPattern(p string) string {
	if i := strings.IndexByte(p, ' '); i >= 0 {
		return p[:i+1] + "/api/v1" + p[i+1:]
	}
	return "/api/v1" + p
}

type routeKey struct{}

func healthz(db Pinger, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, dbStatus := http.StatusOK, "ok"
		if err := db.Ping(ctx); err != nil {
			status, dbStatus = http.StatusServiceUnavailable, "down"
			log.Warn("db ping failed", "reason", SafeErr(err))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": map[bool]string{true: "ok", false: "unavailable"}[status == http.StatusOK],
			"db":     dbStatus,
		})
	}
}

// SafeErr maps an error to a fixed category. Raw error text can contain the
// DSN, host or credentials, so it is never logged.
func SafeErr(err error) string {
	var ne net.Error
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &ne):
		return "network error"
	default:
		return "database error"
	}
}

// route returns the matched mux pattern, never the raw path, so tokens and
// ids in URLs cannot reach the logs.
func route(r *http.Request, sub string) string {
	if sub != "" {
		return sub
	}
	if r.Pattern == "" {
		return "unmatched"
	}
	return r.Pattern
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		var sub string
		r = r.WithContext(context.WithValue(r.Context(), routeKey{}, &sub))
		next.ServeHTTP(sw, r)
		log.Info("request",
			"method", r.Method,
			"route", route(r, sub),
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
