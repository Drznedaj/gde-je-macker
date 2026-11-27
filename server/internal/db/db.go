// Package db wraps the pgx connection pool.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect creates a pool. It does not require the database to be reachable
// yet (pgxpool connects lazily), so /healthz can report 503 instead of the
// process failing to start.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, url)
}
