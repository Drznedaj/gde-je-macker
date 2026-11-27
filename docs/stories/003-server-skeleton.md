# Story 003: Go server skeleton and Postgres wiring

Goal: runnable Go service (config, pgx pool, router, /healthz, request logging, graceful shutdown) that stories 001 and 002 build on.
Owner agent: backend-dev
Files in scope: server/** (go.mod, cmd/server, internal/{config,db,httpapi}), Makefile (add run/test targets), docker-compose.yml (read only)
Out of scope: posts endpoints, alerts, moderation
Do this story FIRST; 001 and 002 depend on it.

## Acceptance criteria
- [ ] `server/go.mod` exists (stdlib net/http router, pgx v5; sqlc config prepared but no generated code needed yet).
- [ ] Config from env (DATABASE_URL, PORT, PHOTO_DIR) with sane local defaults matching docker-compose.
- [ ] GET /healthz returns 200 with db status; fails with 503 if the DB is down.
- [ ] Structured logs without request bodies or contact data.
- [ ] `make run` and `make test` work; CI Go step turns on automatically and passes.
- [ ] Unit test for config parsing and an integration test for /healthz that is skipped when no DB is reachable.

## Report back (under 200 words)
Summary, files touched, open questions.
