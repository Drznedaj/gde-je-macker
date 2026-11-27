package posts

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestBBoxQueryUsesPartialSpatialIndex loads generated posts into a throwaway
// database and checks the plan of the public bbox query. Skipped when the
// dev database is unreachable or may not create databases.
//
//	go test ./internal/posts -run BBoxQuery -v
func TestBBoxQueryUsesPartialSpatialIndex(t *testing.T) {
	const rows = 50000
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, testDSN())
	if err != nil {
		t.Skip("database not available")
	}
	defer admin.Close(ctx)
	const scratch = "macker_explain_scratch"
	admin.Exec(ctx, `DROP DATABASE IF EXISTS `+scratch)
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+scratch); err != nil {
		t.Skip("cannot create scratch database")
	}
	defer admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+scratch)

	u, _ := url.Parse(testDSN())
	u.Path = "/" + scratch
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	mig, err := os.ReadFile("../../../migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.SplitN(strings.SplitN(string(mig), "-- +goose Up", 2)[1], "-- +goose Down", 2)[0]
	if _, err := conn.Exec(ctx, up); err != nil {
		t.Fatal(err)
	}
	// 50% of generated posts are public (active + pending/approved), the rest are
	// expired, held or rejected, spread over Serbia.
	if _, err := conn.Exec(ctx, `
		INSERT INTO posts (kind, species, description, last_seen_point, last_seen_at, contact,
			status, moderation, manage_token_hash, created_at, expires_at)
		SELECT (ARRAY['lost','found'])[1 + i % 2]::post_kind,
		       (ARRAY['dog','cat'])[1 + i % 2]::species, 'x',
		       ST_SetSRID(ST_MakePoint(19.0 + random() * 3.9, 42.4 + random() * 3.6), 4326)::geography,
		       now(), 'contact',
		       CASE WHEN i % 10 = 0 THEN 'expired' ELSE 'active' END::post_status,
		       (ARRAY['pending','approved','approved','held','rejected'])[1 + i % 5]::moderation_state,
		       '\x00', now() - (i || ' seconds')::interval, now() + interval '30 days'
		FROM generate_series(1, $1::int) AS i`, rows); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `ANALYZE posts`); err != nil {
		t.Fatal(err)
	}

	// Same predicate and shape as Store.List, Belgrade-sized viewport.
	r, err := conn.Query(ctx, `EXPLAIN (ANALYZE, BUFFERS) SELECT `+postColumns+` FROM posts WHERE `+publicPredicate+`
		AND last_seen_point && ST_MakeEnvelope(20.3, 44.7, 20.6, 44.9, 4326)::geography
		ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := pgx.CollectRows(r, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(plan, "\n")
	t.Logf("%d rows\n%s", rows, out)
	if !strings.Contains(out, "posts_public_point_gix") {
		t.Fatal("bbox query does not use posts_public_point_gix")
	}
	if strings.Contains(out, "Seq Scan") {
		t.Fatal("bbox query falls back to a sequential scan")
	}
}
