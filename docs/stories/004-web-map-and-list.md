# Story 004: Web map and post list (against a mock API)

Goal: static web page showing lost/found posts on a MapLibre map plus a list, with filters, post detail view and trilingual strings.
Owner agent: frontend-dev
Files in scope: web/**
Out of scope: post creation form (story 005), alerts, backend
Depends on: docs/api/openapi.yaml only; build a mock layer (web/js/api.js with a `?mock=1` switch and fixture JSON).

## Acceptance criteria
- [ ] No framework, no build step; ES modules; served by any static server.
- [ ] MapLibre GL with the pmtiles protocol; a dev fallback style is fine until tiles are hosted (document which).
- [ ] Map loads posts by bbox on move/zoom (debounced); markers differ for lost vs found; clusters when dense.
- [ ] List view synced with the map; filters: kind, species, last 7/30 days.
- [ ] Post detail: photos, description, last-seen date, approximate location (already rounded by API), contact, report button.
- [ ] i18n dictionaries sr-Latn, sr-Cyrl, en with a language switcher stored on the device; default sr-Latn.
- [ ] Responsive at 360 px width; keyboard reachable; alt text for photos.
- [ ] Brand strings come from one config file (app name "Gde je Macker").

## Report back (under 200 words)
Summary, files touched, how to run, open questions.
