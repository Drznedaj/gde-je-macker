package posts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HugoSmits86/nativewebp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultLifetime is how long a post stays active unless renewed.
const DefaultLifetime = 30 * 24 * time.Hour

// publicPredicate repeats the partial-index predicate of
// posts_public_point_gix / posts_public_recent_idx so the planner can use
// them. Held and rejected posts must never match. Keep in sync with the
// migration. The expiry check makes expiry immediate, before the sweeper runs.
const publicPredicate = `status = 'active' AND moderation IN ('pending', 'approved') AND expires_at > now()`

const postColumns = `id, kind::text, species::text, sex::text, neutered::text, goes_outside::text,
	name, description, ST_Y(last_seen_point::geometry), ST_X(last_seen_point::geometry),
	last_seen_at, contact, radius_m, status::text, created_at, expires_at`

var errNotFound = errors.New("not found")

// validationError carries per-field problems found while talking to the store.
type validationError struct{ fields fieldErrors }

func (e validationError) Error() string { return "validation failed" }

// Store is the Postgres-backed posts repository.
type Store struct {
	pool     *pgxpool.Pool
	photoDir string
}

// PhotoRef is the public view of a photo.
type PhotoRef struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Post is the public view of a post. Coordinates are rounded (ADR 0002);
// the exact point never leaves the database layer.
type Post struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Species     string      `json:"species"`
	Sex         string      `json:"sex"`
	Neutered    string      `json:"neutered"`
	GoesOutside string      `json:"goes_outside"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description"`
	Point       publicPoint `json:"last_seen_point"`
	LastSeenAt  time.Time   `json:"last_seen_at"`
	Contact     string      `json:"contact"`
	RadiusM     *int        `json:"radius_m,omitempty"`
	Status      string      `json:"status"`
	Photos      []PhotoRef  `json:"photos"`
	CreatedAt   time.Time   `json:"created_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

type publicPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// round3 rounds to 3 decimals (about 100 m).
func round3(f float64) float64 { return math.Round(f*1000) / 1000 }

func photoURL(id string) string { return "/api/v1/media/" + id + ".webp" }

func (s *Store) photoPath(id string) string { return filepath.Join(s.photoDir, id+".webp") }

// newToken returns a 256-bit base64url token and its SHA-256 hash. The token
// has high entropy, so a fast hash is enough; only the hash is stored.
func newToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

// SavePhoto writes a processed WebP under a fresh id. It is attached to a
// post later through photo_ids.
func (s *Store) SavePhoto(id string, webp []byte) error {
	if err := os.MkdirAll(s.photoDir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.photoDir, "upload-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(webp)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), s.photoPath(id)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (s *Store) removePhotoFiles(ids []string) {
	for _, id := range ids {
		_ = os.Remove(s.photoPath(id))
	}
}

// Create inserts a post and its photos and returns it with the one-time token.
func (s *Store) Create(ctx context.Context, v validated) (Post, string, error) {
	token, hash, err := newToken()
	if err != nil {
		return Post{}, "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, "", err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO posts (kind, species, sex, neutered, goes_outside, name, description,
			last_seen_point, last_seen_at, contact, manage_token_hash, expires_at)
		VALUES ($1::post_kind, $2::species, $3::pet_sex, $4::tri_state, $5::tri_state, NULLIF($6, ''), $7,
			ST_SetSRID(ST_MakePoint($9, $8), 4326)::geography, $10, $11, $12, now() + $13::interval)
		RETURNING id`,
		v.Kind, v.Species, v.Sex, v.Neutered, v.GoesOutside, v.Name, v.Description,
		v.lat, v.lon, *v.LastSeenAt, v.contact, hash, fmt.Sprintf("%d seconds", int(DefaultLifetime.Seconds())),
	).Scan(&id)
	if err != nil {
		return Post{}, "", err
	}
	if _, err := s.setPhotos(ctx, tx, id, v.PhotoIDs); err != nil {
		return Post{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, "", err
	}
	p, err := s.Get(ctx, id, false)
	return p, token, err
}

// setPhotos replaces the photos of a post with ids (in order) and returns the
// ids that were dropped, whose files the caller removes after commit.
func (s *Store) setPhotos(ctx context.Context, tx pgx.Tx, postID string, ids []string) (dropped []string, err error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM post_photos WHERE post_id = $1`, postID)
	if err != nil {
		return nil, err
	}
	old, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM post_photos WHERE post_id = $1`, postID); err != nil {
		return nil, err
	}
	keep := map[string]bool{}
	for i, id := range ids {
		keep[id] = true
		f, err := os.Open(s.photoPath(id))
		if err != nil {
			return nil, validationError{fieldErrors{"photo_ids": "unknown photo " + id}}
		}
		cfg, err := nativewebp.DecodeConfig(f)
		f.Close()
		if err != nil {
			return nil, validationError{fieldErrors{"photo_ids": "unreadable photo " + id}}
		}
		_, err = tx.Exec(ctx, `INSERT INTO post_photos (id, post_id, path, width, height, position)
			VALUES ($1, $2, $3, $4, $5, $6)`, id, postID, id+".webp", cfg.Width, cfg.Height, i)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, validationError{fieldErrors{"photo_ids": "photo already in use " + id}}
		}
		if err != nil {
			return nil, err
		}
	}
	for _, id := range old {
		if !keep[id] {
			dropped = append(dropped, id)
		}
	}
	return dropped, nil
}

func scanPost(row pgx.Row) (Post, error) {
	var p Post
	var name *string
	err := row.Scan(&p.ID, &p.Kind, &p.Species, &p.Sex, &p.Neutered, &p.GoesOutside, &name, &p.Description,
		&p.Point.Lat, &p.Point.Lon, &p.LastSeenAt, &p.Contact, &p.RadiusM, &p.Status, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, errNotFound
	}
	if name != nil {
		p.Name = *name
	}
	p.Point.Lat, p.Point.Lon = round3(p.Point.Lat), round3(p.Point.Lon)
	p.Photos = []PhotoRef{}
	return p, err
}

// Get returns one post. With publicOnly the public predicate applies
// (the post page); without it the owner's own view is returned.
func (s *Store) Get(ctx context.Context, id string, publicOnly bool) (Post, error) {
	q := `SELECT ` + postColumns + ` FROM posts WHERE id = $1`
	if publicOnly {
		q += ` AND ` + publicPredicate
	}
	p, err := scanPost(s.pool.QueryRow(ctx, q, id))
	if err != nil {
		return Post{}, err
	}
	if err := s.loadPhotos(ctx, []*Post{&p}); err != nil {
		return Post{}, err
	}
	return p, nil
}

func (s *Store) loadPhotos(ctx context.Context, ps []*Post) error {
	if len(ps) == 0 {
		return nil
	}
	byID := make(map[string]*Post, len(ps))
	ids := make([]string, 0, len(ps))
	for _, p := range ps {
		byID[p.ID] = p
		ids = append(ids, p.ID)
	}
	rows, err := s.pool.Query(ctx, `SELECT post_id::text, id::text, width, height FROM post_photos
		WHERE post_id = ANY($1::uuid[]) ORDER BY post_id, position`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var ph PhotoRef
		if err := rows.Scan(&pid, &ph.ID, &ph.Width, &ph.Height); err != nil {
			return err
		}
		ph.URL = photoURL(ph.ID)
		byID[pid].Photos = append(byID[pid].Photos, ph)
	}
	return rows.Err()
}

// ListFilter is a validated listing request.
type ListFilter struct {
	West, South, East, North float64
	Kind, Species            string
	Since                    *time.Time
	Limit                    int
}

// List returns public posts in a bounding box, newest first.
func (s *Store) List(ctx context.Context, f ListFilter) ([]Post, error) {
	args := []any{f.West, f.South, f.East, f.North}
	q := `SELECT ` + postColumns + ` FROM posts WHERE ` + publicPredicate +
		` AND last_seen_point && ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography`
	if f.Kind != "" {
		args = append(args, f.Kind)
		q += fmt.Sprintf(` AND kind = $%d::post_kind`, len(args))
	}
	if f.Species != "" {
		args = append(args, f.Species)
		q += fmt.Sprintf(` AND species = $%d::species`, len(args))
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		q += fmt.Sprintf(` AND created_at > $%d`, len(args))
	}
	args = append(args, f.Limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ptrs := make([]*Post, len(out))
	for i := range out {
		ptrs[i] = &out[i]
	}
	return out, s.loadPhotos(ctx, ptrs)
}

// Authorize reports whether token is the manage token of post id. Unknown
// posts and wrong tokens are indistinguishable to the caller.
func (s *Store) Authorize(ctx context.Context, id, token string) (bool, error) {
	if !uuidRe.MatchString(id) || !tokenRe.MatchString(token) {
		return false, nil
	}
	var stored []byte
	err := s.pool.QueryRow(ctx, `SELECT manage_token_hash FROM posts WHERE id = $1`, id).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(stored, hashToken(token)) == 1, nil
}

// Update applies a partial edit and returns the post.
func (s *Store) Update(ctx context.Context, id string, in updateInput, contact string, photoIDs []string) (Post, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE posts SET description = COALESCE($2, description),
		contact = COALESCE($3, contact), updated_at = now() WHERE id = $1`,
		id, trimPtr(in.Description), nilIfEmpty(contact))
	if err != nil {
		return Post{}, err
	}
	if tag.RowsAffected() == 0 {
		return Post{}, errNotFound
	}
	var dropped []string
	if in.PhotoIDs != nil {
		if dropped, err = s.setPhotos(ctx, tx, id, photoIDs); err != nil {
			return Post{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}
	s.removePhotoFiles(dropped)
	return s.Get(ctx, id, false)
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Delete removes the post, its photo rows (cascade) and photo files.
func (s *Store) Delete(ctx context.Context, id string) error {
	rows, err := s.pool.Query(ctx, `DELETE FROM posts WHERE id = $1
		RETURNING (SELECT coalesce(array_agg(id::text), '{}') FROM post_photos WHERE post_id = $1)`, id)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectOneRow(rows, pgx.RowTo[[]string])
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	s.removePhotoFiles(ids)
	return nil
}

// Renew restarts the default lifetime. An expired post becomes active again.
func (s *Store) Renew(ctx context.Context, id string) (Post, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE posts SET expires_at = now() + $2::interval, status = 'active', updated_at = now()
		WHERE id = $1 AND status IN ('active', 'expired')`, id, fmt.Sprintf("%d seconds", int(DefaultLifetime.Seconds())))
	if err != nil {
		return Post{}, err
	}
	if tag.RowsAffected() == 0 {
		return Post{}, errNotFound
	}
	return s.Get(ctx, id, false)
}

// ExpireDue marks active posts past their expiry as expired.
func (s *Store) ExpireDue(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE posts SET status = 'expired', updated_at = now()
		WHERE status = 'active' AND expires_at <= now()`)
	return tag.RowsAffected(), err
}

// PhotoIsPublic reports whether the photo belongs to a post that is
// publicly visible, so photos of held or rejected posts are never served.
func (s *Store) PhotoIsPublic(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM post_photos pp JOIN posts p ON p.id = pp.post_id
		WHERE pp.id = $1 AND p.`+strings.ReplaceAll(publicPredicate, " AND ", " AND p.")+`)`, id).Scan(&ok)
	return ok, err
}
