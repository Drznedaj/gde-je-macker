# CLAUDE.md: project rules for agents

Project: Gde je Macker, a free open-source lost-pet map + alerts app for Serbia. MIT license. Read `README.md` first; deeper background lives in the Claude project docs and `docs/`.

## Non-negotiables
- No personal data: no accounts, no emails/phones in tables, no stored device locations. Alert devices subscribe to coarse geohash cells; exact filtering happens on the device.
- Posts are managed with a secret token (only its hash is stored).
- Strip EXIF from every uploaded photo (re-encode).
- The listing publishes immediately; the push alert is sent only after moderation approves.
- Sightings are NOT in the MVP. The `sightings` table exists; build no UI or endpoints for it yet.
- Stay inside your directories. Never edit files outside your story's listed scope.
- `config/radius.yaml` is owned by the data agent; every value needs an `evidence` note.

## Stack
Go (stdlib net/http or chi, pgx, sqlc, goose-style SQL migrations) · Postgres 16 + PostGIS · vanilla ES-module JS (no framework), MapLibre GL, service worker · Capacitor for native shells.

## Token-saving rules
1. Work from the story file. Read only files it names. No whole-repo scans.
2. Run linters/tests/codegen as commands and read only failures.
3. Report in under 200 words: summary, files touched, open questions. No pasted code.
4. One story per session. State lives in story files, `docs/api/openapi.yaml`, ADRs and Trello.
5. Max two fix rounds per PR, then stop and ask the human.
6. Prefer the standard library and tiny dependencies. Use scripts, not LLM calls, for deterministic work.

## Flow
PM writes `docs/stories/NNN-*.md` -> dev agent implements on a branch -> QA agent tests against the acceptance criteria -> reviewer (read-only) comments -> human merges. Production deploys are manual. Full rules: docs/branching.md.

## Commits
Use the repo-local git identity (GitHub no-reply). Sign off commits (`-s`). Branch yourself: `feature/<story-id>-<short-name>` from the latest `main` (see docs/branching.md). Commit message = one short imperative sentence. Never push and never commit to `main`; the human pushes and opens the PR.

## Local environment
- Docker runs through Rancher Desktop; use `make db-up` / `make db-migrate` (the Makefile detects `docker compose` vs `docker-compose`).
- Agents run in Claude Code on the owner's machine; the PM chat cannot run Docker.

## Checks (once code exists)
- Go: `cd server && go vet ./... && go test ./...`
- SQL: migrations apply cleanly to an empty Postgres+PostGIS.
- Web: no build step required; lint with the configured linter when added.
