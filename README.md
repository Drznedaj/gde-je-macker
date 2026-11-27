# Gde je Macker

Free, open-source map and alerts for lost and found pets in Serbia.

- Post a lost or found pet with photos and a last-seen pin on a map.
- Get an alert when a pet goes missing near you. The alert radius depends on species, whether the pet goes outside, sex and neutering, and urban vs rural (see `config/radius.yaml`).
- No accounts, no personal data stored beyond the posts. Your home point for alerts stays on your device.

Status: early planning and scaffolding. Not usable yet.

## Stack
Vanilla HTML/JS/CSS web app (MapLibre GL, self-hosted PMTiles) · Go API · Postgres + PostGIS · Capacitor shells for Android/iOS · Hetzner hosting.

## Layout
```
server/      Go API, background jobs, moderation pipeline
web/         vanilla JS frontend
mobile/      Capacitor shells (later)
data/        radius calibration and density lookup tooling
infra/       Hetzner, Caddy, backups, deploy
migrations/  SQL migrations
config/      radius.yaml (evidence-backed values)
docs/        API contract (OpenAPI), ADRs, story files
```

## Contributing
See `CONTRIBUTING.md`. Licensed under MIT.

"Gde je Macker" is a name only; this project uses no third-party cartoon characters, artwork or media.
