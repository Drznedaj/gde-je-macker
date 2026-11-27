# Story 011: Optional contact and private messages (API)

Goal: Implement ADR 0003 on the server: contact becomes optional, plus a private message box per post and an optional owner push.
Owner agent: backend-dev
Files in scope: `migrations/0002_*.sql` (new), `server/**`, `docs/api/openapi.yaml`
Out of scope: web UI (story 012), sightings, alerts to nearby devices
Depends on: ADR 0003 accepted; story 001 merged (posts API, manage-token auth); story 009 merged (`/api/v1` mux)

## Acceptance criteria
- [ ] New migration: `posts.contact` is nullable, and the 3-200 length check applies only when it's present. Add table `post_messages` (id, post_id FK ON DELETE CASCADE, body 1-500, moderation state, created_at, read_at) and table `post_owner_push` (post_id PK FK ON DELETE CASCADE, transport, endpoint, p256dh, auth, created_at) with the same transport checks as `push_subscriptions`. The Down section reverses it.
- [ ] `POST /posts/{id}/messages` (public): body only. Returns 404 for held, rejected or inactive posts. Returns 422 on limits. Rate limited per post and per client in memory, returning 429. No IP or user agent is stored or logged.
- [ ] New messages go through the automated text check. Only `pending`/`approved` messages are visible to the owner, and held ones wait for a human.
- [ ] `GET /posts/{id}/messages` (manage token): newest first, and marks them read. `DELETE /posts/{id}/messages/{mid}` (manage token). Both return 401 on a wrong token, and a message from another post returns 404.
- [ ] `PUT`/`DELETE /posts/{id}/owner-push` (manage token) stores or removes one endpoint. On a new visible message, send a push with no message text ("New message about your post"). A failed or expired endpoint is deleted.
- [ ] The post response omits `contact` when it's null, and the create/update schemas mark it optional. Update openapi.yaml for every new path and field.
- [ ] Deleting a post or the retention sweep removes its messages and owner push (cascade test).

## Test notes
- Wrong token, a cross-post message id, a held post, the rate limit, and the cascade on delete.
- Assert that no request logs contain the message body.
- `make test` and `make db-reset` pass.

## Report back (under 200 words)
Summary, files touched, open questions.
