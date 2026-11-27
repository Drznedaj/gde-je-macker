# Story 001: Posts API and photo upload

Goal: implement POST/GET /posts, GET/PATCH/DELETE /posts/{id}, /photos, with expiry and manage-token auth, per docs/api/openapi.yaml.
Owner agent: backend-dev
Files in scope: server/**, migrations/** (read only unless a migration bug is found)
Out of scope: alerts, moderation, sightings, frontend

## Acceptance criteria
- [ ] Create a post returns a manage token once; only its hash is stored.
- [ ] Points outside Serbia are rejected (422).
- [ ] Photos are re-encoded to WebP, resized, EXIF removed; max 3 per post.
- [ ] Bbox listing uses the PostGIS index and excludes expired, removed and rejected posts.
- [ ] Wrong or missing token gives 401 on PATCH/DELETE/renew.
- [ ] Posts expire automatically (default 30 days).

## Test notes
- Include a photo with GPS EXIF and assert it is gone from the stored file.
