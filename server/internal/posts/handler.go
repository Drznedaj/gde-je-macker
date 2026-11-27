// Package posts implements the posts and photos API (story 001).
package posts

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Drznedaj/gde-je-macker/server/internal/httpapi"
	"github.com/Drznedaj/gde-je-macker/server/internal/photos"
)

const (
	maxJSONBytes = 64 << 10
	// Photo uploads and re-encoding get longer deadlines than the server-wide
	// defaults, which stay in force for every other route.
	photoReadDeadline  = 60 * time.Second
	photoWriteDeadline = 60 * time.Second

	defaultLimit = 100
	maxLimit     = 500
)

// Handler serves the posts API.
type Handler struct {
	store *Store
	log   *slog.Logger
	now   func() time.Time
}

// New creates a Handler that keeps photos under photoDir.
func New(pool *pgxpool.Pool, photoDir string, log *slog.Logger) *Handler {
	return &Handler{store: &Store{pool: pool, photoDir: photoDir}, log: log, now: time.Now}
}

// Mount registers the routes on the /api/v1 mux (paths are relative to it).
func (h *Handler) Mount(v1 *http.ServeMux) {
	v1.HandleFunc("POST /photos", h.uploadPhoto)
	v1.HandleFunc("GET /media/{file}", h.servePhoto)
	v1.HandleFunc("GET /posts", h.list)
	v1.HandleFunc("POST /posts", h.create)
	v1.HandleFunc("GET /posts/{id}", h.get)
	v1.HandleFunc("PATCH /posts/{id}", h.update)
	v1.HandleFunc("DELETE /posts/{id}", h.delete)
	v1.HandleFunc("POST /posts/{id}/renew", h.renew)
}

// RunExpiry marks posts expired every interval until ctx ends.
func (h *Handler) RunExpiry(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if n, err := h.store.ExpireDue(ctx); err != nil && ctx.Err() == nil {
			h.log.Warn("expiry sweep failed", "reason", httpapi.SafeErr(err))
		} else if n > 0 {
			h.log.Info("posts expired", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeValidation(w http.ResponseWriter, errs fieldErrors) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_failed", "fields": errs})
}

func (h *Handler) internal(w http.ResponseWriter, what string, err error) {
	h.log.Error(what, "reason", httpapi.SafeErr(err))
	writeErr(w, http.StatusInternalServerError, "internal_error")
}

// decode reads a small JSON body; it writes the error response itself.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBytes))
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		} else {
			writeErr(w, http.StatusBadRequest, "invalid_json")
		}
		return false
	}
	return true
}

func (h *Handler) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(photoReadDeadline)) // ErrNotSupported is fine (tests)
	_ = rc.SetWriteDeadline(time.Now().Add(photoWriteDeadline))

	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxUploadBytes+(1<<20))
	file, _, err := r.FormFile("file")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		} else {
			writeErr(w, http.StatusUnsupportedMediaType, "file_required")
		}
		return
	}
	defer file.Close()
	res, err := photos.Process(file)
	switch {
	case errors.Is(err, photos.ErrTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case errors.Is(err, photos.ErrUnsupported), errors.Is(err, photos.ErrCorrupt):
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_type")
		return
	case err != nil:
		h.internal(w, "photo processing failed", err)
		return
	}
	id, err := newUUID()
	if err == nil {
		err = h.store.SavePhoto(id, res.WebP)
	}
	if err != nil {
		h.internal(w, "photo save failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, PhotoRef{ID: id, URL: photoURL(id), Width: res.Width, Height: res.Height})
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func (h *Handler) servePhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := strings.CutSuffix(r.PathValue("file"), ".webp")
	if !ok || !uuidRe.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	public, err := h.store.PhotoIsPublic(r.Context(), id)
	if err != nil {
		h.internal(w, "photo lookup failed", err)
		return
	}
	f, ferr := os.Open(h.store.photoPath(id))
	if !public || ferr != nil {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, "", time.Time{}, f)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if !decode(w, r, &in) {
		return
	}
	v, errs := in.validate(h.now())
	if len(errs) > 0 {
		writeValidation(w, errs)
		return
	}
	p, token, err := h.store.Create(r.Context(), v)
	var ve validationError
	switch {
	case errors.As(err, &ve):
		writeValidation(w, ve.fields)
		return
	case err != nil:
		h.internal(w, "create post failed", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, struct {
		Post
		ManageToken string `json:"manage_token"`
	}{p, token})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	p, err := h.store.Get(r.Context(), id, true)
	switch {
	case errors.Is(err, errNotFound):
		writeErr(w, http.StatusNotFound, "not_found")
	case err != nil:
		h.internal(w, "get post failed", err)
	default:
		writeJSON(w, http.StatusOK, p)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	f, errs := parseList(r)
	if len(errs) > 0 {
		writeValidation(w, errs)
		return
	}
	ps, err := h.store.List(r.Context(), f)
	if err != nil {
		h.internal(w, "list posts failed", err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func parseList(r *http.Request) (ListFilter, fieldErrors) {
	q := r.URL.Query()
	errs := fieldErrors{}
	f := ListFilter{Limit: defaultLimit}
	parts := strings.Split(q.Get("bbox"), ",")
	var c [4]float64
	ok := len(parts) == 4
	for i := 0; ok && i < 4; i++ {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		ok = err == nil && v >= -180 && v <= 180 // also rejects NaN
		c[i] = v
	}
	if !ok || c[1] < -90 || c[3] > 90 || c[0] >= c[2] || c[1] >= c[3] {
		errs["bbox"] = "must be west,south,east,north"
	}
	f.West, f.South, f.East, f.North = c[0], c[1], c[2], c[3]
	if k := q.Get("kind"); k != "" {
		validateEnum("kind", k, kinds, errs)
		f.Kind = k
	}
	if s := q.Get("species"); s != "" {
		validateEnum("species", s, speciesOK, errs)
		f.Species = s
	}
	if s := q.Get("since"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			errs["since"] = "must be ISO 8601"
		}
		f.Since = &t
	}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxLimit {
			errs["limit"] = fmt.Sprintf("must be 1-%d", maxLimit)
		}
		f.Limit = n
	}
	return f, errs
}

// authorize checks the X-Manage-Token header. Wrong token, malformed token
// (including any containing "/" or "%2F") and unknown post all give 401.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	ok, err := h.store.Authorize(r.Context(), id, r.Header.Get("X-Manage-Token"))
	if err != nil {
		h.internal(w, "authorize failed", err)
		return "", false
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return id, true
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var in updateInput
	if !decode(w, r, &in) {
		return
	}
	errs := fieldErrors{}
	if in.Description == nil && in.Contact == nil && in.PhotoIDs == nil {
		errs["body"] = "nothing to update"
	}
	if in.Description != nil {
		validateText("description", strings.TrimSpace(*in.Description), maxDescription, errs)
	}
	var contact string
	if in.Contact != nil {
		var problem string
		if contact, problem = validateContact(in.Contact); problem != "" {
			errs["contact"] = problem
		}
	}
	var photoIDs []string
	if in.PhotoIDs != nil {
		photoIDs = validatePhotoIDs(*in.PhotoIDs, errs)
	}
	if len(errs) > 0 {
		writeValidation(w, errs)
		return
	}
	p, err := h.store.Update(r.Context(), id, in, contact, photoIDs)
	var ve validationError
	switch {
	case errors.As(err, &ve):
		writeValidation(w, ve.fields)
	case errors.Is(err, errNotFound):
		writeErr(w, http.StatusUnauthorized, "unauthorized")
	case err != nil:
		h.internal(w, "update post failed", err)
	default:
		writeJSON(w, http.StatusOK, p)
	}
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil && !errors.Is(err, errNotFound) {
		h.internal(w, "delete post failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) renew(w http.ResponseWriter, r *http.Request) {
	id, ok := h.authorize(w, r)
	if !ok {
		return
	}
	p, err := h.store.Renew(r.Context(), id)
	switch {
	case errors.Is(err, errNotFound):
		writeErr(w, http.StatusConflict, "not_renewable")
	case err != nil:
		h.internal(w, "renew post failed", err)
	default:
		writeJSON(w, http.StatusOK, p)
	}
}
