// Package httpapi holds the HTTP router and middleware.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the HTTP handler. Requests are logged without bodies,
// query strings, raw paths or client addresses.
func NewRouter(db Pinger, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(db, log))
	return logging(log, mux)
}

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
func route(r *http.Request) string {
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
		next.ServeHTTP(sw, r)
		log.Info("request",
			"method", r.Method,
			"route", route(r),
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
