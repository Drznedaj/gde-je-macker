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
1. Work from the story file (`../pm/stories/NNN-*.md`). Read only files it names. No whole-repo scans.
2. Run linters/tests/codegen as commands and read only failures.
3. Report in under 200 words to `../pm/reports/NNN.md`: summary, files touched, open questions. No pasted code.
4. One story per session. State lives in `../pm/` (stories, reports), `docs/api/openapi.yaml`, ADRs and Trello.
5. Max two fix rounds per PR, then stop and ask the human.
6. Prefer the standard library and tiny dependencies. Use scripts, not LLM calls, for deterministic work.

## Flow
PM writes `../pm/stories/NNN-*.md` -> dev agent implements on a branch -> QA agent tests against the acceptance criteria -> reviewer (read-only) comments -> auto-merge when checks and verdicts pass, otherwise the human. Run it with `/story <story-id>` (`.claude/commands/story.md`). Production deploys are manual. Full rules: `../pm/WORKFLOW.md` and docs/branching.md. Several stories can run at once, each in its own worktree (../pm/WORKFLOW.md section 8)

### Where things live
- `../pm/` (next to the repo, never committed): stories, reports, spike notes, ADR drafts.
- Repo: code, migrations, `docs/api/openapi.yaml`, accepted ADRs, `CLAUDE.md`, CI, `config/radius.yaml`.
- Trello holds status and decisions; Claude project docs hold long-form research.

### Story lifecycle
1. Owner runs `/story NNN`; the agent reads `../pm/stories/NNN-*.md` and works on `feature/NNN-*`.
2. The agent writes its report to `../pm/reports/NNN.md` (not committed, never in a PR).
3. QA and reviewer post PASS/FAIL as PR comments. Max 2 fix rounds, then stop and ask the PM.

### PR and merge rules
- PRs are for code only. No PR for stories, reports, spikes or ADR drafts; an ADR ships in the PR of the story that implements it.
- Auto-merge (squash) when both verdicts PASS and CI is green.
- Label `needs-human` and wait when the diff touches `migrations/`, `.github/`, `.claude/`, `CLAUDE.md`, `config/radius.yaml`, `docs/adr/`, `LICENSE`, or code handling manage tokens, photo storage or contact/message data.

## Commits
Use the repo-local git identity (GitHub no-reply). Sign off commits (`-s`). Branch yourself: `feature/<story-id>-<short-name>` from the latest `main` (see docs/branching.md). Commit message = one short imperative sentence. Agents may push `feature/*` branches and open PRs; never push or commit to `main`, never force-push, never merge with admin override.

## Local environment
- Docker runs through Rancher Desktop; use `make db-up` / `make db-migrate` (the Makefile detects `docker compose` vs `docker-compose`).
- Agents run in Claude Code on the owner's machine; the PM chat cannot run Docker.

## Checks (once code exists)
- Go: `cd server && go vet ./... && go test ./...`
- SQL: migrations apply cleanly to an empty Postgres+PostGIS.
- Web: no build step required; lint with the configured linter when added.
