# Web (story 004)

Static site: plain HTML, CSS and ES modules. No build step.

Run with mock data: `cd web && python3 -m http.server 8080`, then open `http://localhost:8080/?mock=1`.
Without `?mock=1` the client calls `/api/v1/posts` (see docs/api/openapi.yaml).

Map: MapLibre GL and pmtiles are loaded from unpkg. Until tiles are hosted, `js/config.js` has an empty
`pmtilesUrl` and the dev fallback is public OpenStreetMap raster tiles (dev use only). Brand strings and map settings live in `js/config.js`; texts in `i18n/*.json`.
