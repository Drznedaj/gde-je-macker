# ADR 0001: Stack

Status: accepted (2026-11-27)

- Frontend: vanilla HTML/JS/CSS (ES modules), MapLibre GL with self-hosted PMTiles. No framework.
- Backend: Go, Postgres 16 + PostGIS, background jobs in the same binary.
- Native: Capacitor shells around the same static web app (iOS web push only works from the Home Screen, so a native shell is needed for reliable iPhone alerts).
- Hosting: one Hetzner VM (Caddy, Go, Postgres), Storage Box backups. No Supabase or other paid-per-seat services.
- Alerts: devices subscribe to coarse geohash cells; transport (FCM topics vs self-hosted Web Push) decided by a spike in Sprint 2.
- License: MIT. Contributions use DCO sign-off.
