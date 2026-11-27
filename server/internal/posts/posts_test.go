package posts

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Drznedaj/gde-je-macker/server/internal/httpapi"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// ---- tests that need no database ----

func TestValidation(t *testing.T) {
	now := time.Now()
	ok := func() createInput {
		lat, lon := 44.8, 20.4
		c, at := "064 123 4567", now.Add(-time.Hour)
		return createInput{Kind: "lost", Species: "dog", Point: &pointJSON{&lat, &lon}, LastSeenAt: &at, Contact: &c}
	}
	if _, errs := ok().validate(now); len(errs) != 0 {
		t.Fatalf("valid input rejected: %v", errs)
	}
	long := func(n int) string { return strings.Repeat("ž", n) } // runes, not bytes
	cases := map[string]func(*createInput){
		"point outside Serbia": func(in *createInput) { lat := 48.85; in.Point.Lat = &lat },
		"name 81":              func(in *createInput) { in.Name = long(81) },
		"description 2001":     func(in *createInput) { in.Description = long(2001) },
		"contact 2":            func(in *createInput) { c := "ab"; in.Contact = &c },
		"contact 201":          func(in *createInput) { c := long(201); in.Contact = &c },
		"contact missing":      func(in *createInput) { in.Contact = nil },
		"bad kind":             func(in *createInput) { in.Kind = "bird" },
		"4 photos": func(in *createInput) {
			id := "00000000-0000-4000-8000-000000000001"
			in.PhotoIDs = []string{id, id, id, id}
		},
		"future last_seen": func(in *createInput) { at := now.Add(time.Hour); in.LastSeenAt = &at },
	}
	for name, mut := range cases {
		in := ok()
		mut(&in)
		if _, errs := in.validate(now); len(errs) == 0 {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	for _, c := range []string{long(80), strings.Repeat("a", 3)} {
		in := ok()
		in.Name, in.Contact = long(80), &c
		if _, errs := in.validate(now); len(errs) != 0 {
			t.Errorf("boundary values rejected: %v", errs)
		}
	}
}

func TestRound3(t *testing.T) {
	if got := round3(44.81234); got != 44.812 {
		t.Fatalf("got %v", got)
	}
	if got := round3(20.4616); got != 20.462 {
		t.Fatalf("got %v", got)
	}
}

func noDBRouter() http.Handler {
	h := &Handler{store: &Store{photoDir: os.TempDir()}, log: quiet, now: time.Now}
	return httpapi.NewRouter(nil, quiet, h.Mount)
}

func TestManageTokenRejectsSlashAndEncodedSlash(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	srv := noDBRouter()
	good := strings.Repeat("A", 43)
	for _, tok := range []string{"", "abc", good + "/", "abc/" + good[4:], good[:20] + "%2F" + good[23:], good[:20] + "%2f" + good[23:]} {
		for _, call := range []struct{ method, path string }{
			{"PATCH", "/api/v1/posts/" + id},
			{"DELETE", "/api/v1/posts/" + id},
			{"POST", "/api/v1/posts/" + id + "/renew"},
		} {
			req := httptest.NewRequest(call.method, call.path, strings.NewReader(`{"description":"x"}`))
			if tok != "" {
				req.Header.Set("X-Manage-Token", tok)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %q: got %d, want 401", call.method, tok, rec.Code)
			}
		}
	}
	// A %2F in the path is decoded before routing and must not reach a token route.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/v1/posts/"+id+"%2F"+strings.Repeat("A", 43), nil)
	srv.ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Errorf("encoded slash in path got %d", rec.Code)
	}
}

// deadlineWriter records the deadlines set through http.ResponseController.
type deadlineWriter struct {
	*httptest.ResponseRecorder
	read, write time.Time
}

func (d *deadlineWriter) SetReadDeadline(t time.Time) error  { d.read = t; return nil }
func (d *deadlineWriter) SetWriteDeadline(t time.Time) error { d.write = t; return nil }

func TestPhotoRouteGetsLongerDeadlines(t *testing.T) {
	h := &Handler{store: &Store{photoDir: t.TempDir()}, log: quiet, now: time.Now}
	mux := http.NewServeMux()
	h.Mount(mux)

	up := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	mux.ServeHTTP(up, httptest.NewRequest("POST", "/photos", strings.NewReader("x")))
	if d := time.Until(up.read); d < 30*time.Second {
		t.Errorf("photo read deadline too short: %v", d)
	}
	if d := time.Until(up.write); d < 30*time.Second {
		t.Errorf("photo write deadline too short: %v", d)
	}

	other := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	mux.ServeHTTP(other, httptest.NewRequest("POST", "/posts", strings.NewReader("{")))
	if !other.read.IsZero() || !other.write.IsZero() {
		t.Error("only the photo route may change deadlines")
	}
}

// ---- integration tests (need the dev Postgres from `make db-up`) ----

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://macker:macker@127.0.0.1:5432/macker?sslmode=disable"
}

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	srv  *httptest.Server
	dir  string
	h    *Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, testDSN())
	if err == nil {
		err = pool.Ping(ctx)
	}
	if err != nil {
		t.Skip("database not available (run make db-up): integration test skipped")
	}
	t.Cleanup(pool.Close)
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_name = 'posts'`).Scan(&n); err != nil || n == 0 {
		t.Skip("migrations not applied (run make db-migrate): integration test skipped")
	}
	dir := t.TempDir()
	h := New(pool, dir, quiet)
	srv := httptest.NewServer(httpapi.NewRouter(pool, quiet, h.Mount))
	t.Cleanup(srv.Close)
	return &env{t: t, pool: pool, srv: srv, dir: dir, h: h}
}

func (e *env) do(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rdr)
	if token != "" {
		req.Header.Set("X-Manage-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (e *env) upload(data []byte) (int, PhotoRef) {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "cat.jpg")
	fw.Write(data)
	mw.Close()
	resp, err := http.Post(e.srv.URL+"/api/v1/photos", mw.FormDataContentType(), &buf)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var ref PhotoRef
	_ = json.NewDecoder(resp.Body).Decode(&ref)
	return resp.StatusCode, ref
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return withGPSEXIF(buf.Bytes())
}

const gpsSentinel = "GPSLatitude=44.8125 SENTINEL-4471"

// withGPSEXIF inserts an APP1 segment carrying the sentinel after SOI.
func withGPSEXIF(j []byte) []byte {
	seg := append([]byte("Exif\x00\x00MM\x00\x2a\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00"), gpsSentinel...)
	n := len(seg) + 2
	out := append([]byte{}, j[:2]...)
	out = append(out, 0xFF, 0xE1, byte(n>>8), byte(n))
	out = append(out, seg...)
	return append(out, j[2:]...)
}

func validPost(photoIDs ...string) map[string]any {
	p := map[string]any{
		"kind": "lost", "species": "cat", "name": "Maca", "description": "Siva macka",
		"last_seen_point": map[string]float64{"lat": 44.81234, "lon": 20.46123},
		"last_seen_at":    time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		"contact":         "064 123 4567",
	}
	if photoIDs != nil {
		p["photo_ids"] = photoIDs
	}
	return p
}

func (e *env) create(body map[string]any) (PostWithToken, int) {
	e.t.Helper()
	code, b := e.do("POST", "/api/v1/posts", "", body)
	var p PostWithToken
	_ = json.Unmarshal(b, &p)
	if code == http.StatusCreated {
		e.t.Cleanup(func() { e.pool.Exec(context.Background(), `DELETE FROM posts WHERE id = $1`, p.ID) })
	}
	return p, code
}

type PostWithToken struct {
	Post
	ManageToken string `json:"manage_token"`
}

func TestLifecycle(t *testing.T) {
	e := newEnv(t)

	// Photo: GPS EXIF must not survive in the stored file.
	code, ref := e.upload(jpegBytes(t))
	if code != http.StatusCreated || ref.ID == "" {
		t.Fatalf("upload: %d", code)
	}
	stored, err := os.ReadFile(filepath.Join(e.dir, ref.ID+".webp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(stored, []byte("RIFF")) || bytes.Contains(stored, []byte("SENTINEL")) || bytes.Contains(stored, []byte("Exif")) {
		t.Fatal("stored photo is not a clean WebP")
	}

	// Create: token shown once, only its hash stored.
	p, code := e.create(validPost(ref.ID))
	if code != http.StatusCreated || len(p.ManageToken) != 43 {
		t.Fatalf("create: %d", code)
	}
	var hash []byte
	if err := e.pool.QueryRow(context.Background(), `SELECT manage_token_hash FROM posts WHERE id = $1`, p.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if string(hash) == p.ManageToken || !bytes.Equal(hash, hashToken(p.ManageToken)) {
		t.Fatal("token must be stored hashed")
	}
	if p.Point.Lat != 44.812 || p.Point.Lon != 20.461 {
		t.Fatalf("point not rounded: %+v", p.Point)
	}
	if d := time.Until(p.ExpiresAt); d < 29*24*time.Hour || d > 31*24*time.Hour {
		t.Fatalf("default expiry %v", d)
	}

	// Public get: no token, rounded point, photo served.
	code, b := e.do("GET", "/api/v1/posts/"+p.ID, "", nil)
	if code != 200 || bytes.Contains(b, []byte("manage_token")) || bytes.Contains(b, []byte("44.81234")) {
		t.Fatalf("get: %d %s", code, b)
	}
	resp, err := http.Get(e.srv.URL + ref.URL)
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/webp" {
		t.Fatalf("photo not served: %v %v", err, resp)
	}
	resp.Body.Close()

	// List by bbox.
	_, b = e.do("GET", "/api/v1/posts?bbox=20.4,44.8,20.5,44.85&species=cat&kind=lost", "", nil)
	var list []Post
	_ = json.Unmarshal(b, &list)
	found := false
	for _, x := range list {
		found = found || x.ID == p.ID
	}
	if !found {
		t.Fatalf("post missing from listing: %s", b)
	}

	// Token checks.
	if c, _ := e.do("PATCH", "/api/v1/posts/"+p.ID, "", map[string]string{"description": "x"}); c != 401 {
		t.Fatalf("missing token: %d", c)
	}
	wrong := strings.Repeat("B", 43)
	for _, tok := range []string{wrong, p.ManageToken + "/", p.ManageToken[:10] + "%2F" + p.ManageToken[13:]} {
		if c, _ := e.do("PATCH", "/api/v1/posts/"+p.ID, tok, map[string]string{"description": "x"}); c != 401 {
			t.Fatalf("bad token %q: %d", tok, c)
		}
	}
	if c, _ := e.do("POST", "/api/v1/posts/"+p.ID+"/renew", wrong, nil); c != 401 {
		t.Fatalf("renew wrong token: %d", c)
	}
	if c, _ := e.do("DELETE", "/api/v1/posts/"+p.ID, wrong, nil); c != 401 {
		t.Fatalf("delete wrong token: %d", c)
	}

	// Edit, renew, delete with the right token.
	if c, b := e.do("PATCH", "/api/v1/posts/"+p.ID, p.ManageToken, map[string]string{"description": "Nova opis"}); c != 200 || !bytes.Contains(b, []byte("Nova opis")) {
		t.Fatalf("patch: %d %s", c, b)
	}
	if c, _ := e.do("PATCH", "/api/v1/posts/"+p.ID, p.ManageToken, map[string]any{"photo_ids": []string{}}); c != 200 {
		t.Fatalf("patch photos: %d", c)
	}
	if _, err := os.Stat(filepath.Join(e.dir, ref.ID+".webp")); !os.IsNotExist(err) {
		t.Fatal("dropped photo file should be deleted")
	}
	if c, _ := e.do("PATCH", "/api/v1/posts/"+p.ID, p.ManageToken, map[string]string{"contact": "x"}); c != 422 {
		t.Fatalf("patch bad contact: %d", c)
	}
	e.pool.Exec(context.Background(), `UPDATE posts SET expires_at = now() + interval '1 day' WHERE id = $1`, p.ID)
	if c, b := e.do("POST", "/api/v1/posts/"+p.ID+"/renew", p.ManageToken, nil); c != 200 {
		t.Fatalf("renew: %d %s", c, b)
	}
	var left time.Duration
	e.pool.QueryRow(context.Background(), `SELECT expires_at - now() FROM posts WHERE id = $1`, p.ID).Scan(&left)
	if left < 29*24*time.Hour {
		t.Fatalf("renew did not extend expiry: %v", left)
	}
	if c, _ := e.do("DELETE", "/api/v1/posts/"+p.ID, p.ManageToken, nil); c != 204 {
		t.Fatalf("delete: %d", c)
	}
	if c, _ := e.do("GET", "/api/v1/posts/"+p.ID, "", nil); c != 404 {
		t.Fatalf("get after delete: %d", c)
	}
}

func TestHeldAndRejectedAreHidden(t *testing.T) {
	e := newEnv(t)
	_, ref := e.upload(jpegBytes(t))
	p, code := e.create(validPost(ref.ID))
	if code != 201 {
		t.Fatal(code)
	}
	for _, state := range []string{"held", "rejected"} {
		e.pool.Exec(context.Background(), `UPDATE posts SET moderation = $2::moderation_state WHERE id = $1`, p.ID, state)
		if c, _ := e.do("GET", "/api/v1/posts/"+p.ID, "", nil); c != 404 {
			t.Errorf("%s: post page %d", state, c)
		}
		_, b := e.do("GET", "/api/v1/posts?bbox=20.4,44.8,20.5,44.85", "", nil)
		if bytes.Contains(b, []byte(p.ID)) {
			t.Errorf("%s: post in listing", state)
		}
		if resp, err := http.Get(e.srv.URL + ref.URL); err != nil || resp.StatusCode != 404 {
			t.Errorf("%s: photo still served", state)
		}
	}
	// The owner can still reach it with the token.
	if c, _ := e.do("PATCH", "/api/v1/posts/"+p.ID, p.ManageToken, map[string]string{"description": "x"}); c != 200 {
		t.Errorf("owner patch on held post: %d", c)
	}
}

func TestCreateValidationOverHTTP(t *testing.T) {
	e := newEnv(t)
	out := validPost()
	out["last_seen_point"] = map[string]float64{"lat": 48.85, "lon": 2.35} // Paris
	if _, code := e.create(out); code != 422 {
		t.Errorf("outside Serbia: %d", code)
	}
	bad := validPost()
	bad["contact"] = "ab"
	if _, code := e.create(bad); code != 422 {
		t.Errorf("short contact: %d", code)
	}
	unknown := validPost("00000000-0000-4000-8000-0000000000aa")
	if _, code := e.create(unknown); code != 422 {
		t.Errorf("unknown photo: %d", code)
	}
}

func TestMaxThreePhotosAndNoReuse(t *testing.T) {
	e := newEnv(t)
	var ids []string
	for i := 0; i < 4; i++ {
		_, ref := e.upload(jpegBytes(t))
		ids = append(ids, ref.ID)
	}
	if _, code := e.create(validPost(ids...)); code != 422 {
		t.Errorf("4 photos: %d", code)
	}
	p, code := e.create(validPost(ids[:3]...))
	if code != 201 || len(p.Photos) != 3 {
		t.Fatalf("3 photos: %d %d", code, len(p.Photos))
	}
	if _, code := e.create(validPost(ids[0])); code != 422 {
		t.Errorf("photo reuse: %d", code)
	}
}

func TestUploadRejectsNonImages(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.upload([]byte("<script>alert(1)</script>")); code != http.StatusUnsupportedMediaType {
		t.Fatalf("got %d", code)
	}
}

func TestExpiry(t *testing.T) {
	e := newEnv(t)
	p, _ := e.create(validPost())
	e.pool.Exec(context.Background(), `UPDATE posts SET created_at = now() - interval '31 days', expires_at = now() - interval '1 day' WHERE id = $1`, p.ID)
	if c, _ := e.do("GET", "/api/v1/posts/"+p.ID, "", nil); c != 404 {
		t.Fatalf("expired post visible: %d", c)
	}
	if _, err := e.h.store.ExpireDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status string
	e.pool.QueryRow(context.Background(), `SELECT status::text FROM posts WHERE id = $1`, p.ID).Scan(&status)
	if status != "expired" {
		t.Fatalf("status %q", status)
	}
	if c, _ := e.do("POST", "/api/v1/posts/"+p.ID+"/renew", p.ManageToken, nil); c != 200 {
		t.Fatalf("renew expired: %d", c)
	}
	if c, _ := e.do("GET", "/api/v1/posts/"+p.ID, "", nil); c != 200 {
		t.Fatalf("renewed post hidden: %d", c)
	}
}
