# Story 009: Serve the public API under /api/v1

Goal: Make the spec and the Go server match the web client. The client calls `/api/v1/...` (story 004), but the spec and router currently serve at the root.
Owner agent: backend-dev
Files in scope: `docs/api/openapi.yaml`, `server/internal/httpapi/**`
Out of scope: `web/**` (the client already uses `/api/v1`), new endpoints (story 001), migrations, deploy/proxy config

## Acceptance criteria
- [ ] `openapi.yaml`: `servers` URL ends in `/api/v1`. Every path except `/healthz` is relative to it.
- [ ] `/healthz` stays at the root, unversioned, for load balancers and uptime checks. In the spec it gets a path-level `servers` override pointing at the root, and its description says so.
- [ ] Router: one versioned sub-mux mounted at `/api/v1/`, so story 001 handlers register on it without repeating the prefix. `GET /healthz` stays on the root mux.
- [ ] Unknown paths under `/api/v1/` return a JSON 404. Root paths other than `/healthz` return 404.
- [ ] The request logging middleware covers both muxes and still never logs bodies, query strings or client addresses.
- [ ] OpenAPI validation in CI still passes.

## Test notes
- httpapi tests: `GET /healthz` returns 200. `GET /api/v1/healthz` and `GET /posts` return 404. `GET /api/v1/nope` returns a JSON 404.
- `make test` passes.

## Report back (under 200 words)
Summary, files touched, open questions. Add a note for story 001 to mount its handlers on the versioned mux.
