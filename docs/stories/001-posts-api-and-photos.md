# Story 001: Posts API and photo upload

Goal: implement POST/GET /posts, GET/PATCH/DELETE /posts/{id}, /photos, with expiry and manage-token auth, per docs/api/openapi.yaml.
Owner agent: backend-dev
Files in scope: server/**, migrations/** (read only unless a migration bug is found)
Out of scope: alerts, moderation, sightings, frontend

## Acceptance criteria
- [ ] Create a post returns a manage token once; only its hash is stored.
- [ ] Points outside Serbia are rejected (422).
- [ ] Photos are re-encoded to WebP, resized, EXIF removed; max 3 per post.
- [ ] Public queries repeat the partial-index predicate `status='active' AND moderation IN ('pending','approved')`; held and rejected posts never appear in listings or on the post page.
- [ ] Public responses round the last-seen point to 3 decimals (about 100 m); the exact point is used only internally for radius and alerts.
- [ ] Field limits from the schema (name 80, description 2000, contact 3-200) are validated in the API with clear 422 errors before the database does.
- [ ] Check with EXPLAIN on a realistic volume (for example 50k generated posts) that the bbox query uses the spatial partial index.
- [ ] Wrong or missing token gives 401 on PATCH/DELETE/renew.
- [ ] Posts expire automatically (default 30 days).

## Test notes
- Include a photo with GPS EXIF and assert it is gone from the stored file.
