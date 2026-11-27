// Package db wraps the pgx connection pool.
package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect creates a pool. It does not require the database to be reachable
// yet (pgxpool connects lazily), so /healthz can report 503 instead of the
// process failing to start.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		// pgx errors can echo parts of the DSN; do not propagate them.
		return nil, errors.New("invalid database configuration")
	}
	return pool, nil
}
