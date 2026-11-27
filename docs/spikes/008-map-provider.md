# Spike 008: map provider for the web map

Status: findings, time-boxed (2026-11-27). No app code changed.
Context: story 004 uses public OSM raster tiles and loads MapLibre, pmtiles and glyphs from unpkg / demotiles.maplibre.org. Both are dev-only (OSM tile policy, no SRI). Constraint: no personal data (CLAUDE.md, ADR 0002).

## Comparison

| | (a) Google Maps Platform | (b) Self-hosted Protomaps (pmtiles) | (c) MapTiler / Stadia (hosted vector) |
|---|---|---|---|
| Cost | Billing account required even for free use [G1]. Free per month: 10k Dynamic Maps loads (JS API), 100k Map Tiles API 2D tiles; then $7.00/1k loads, $0.60/1k tiles [G2]. A viral lost-pet post can blow past caps with no hard stop unless quotas are set. | Static file hosting only. Serbia extract is one file (est. low hundreds of MB to z15, smaller with `--maxzoom`) on any host with HTTP Range support [P2, P3]. Egress cost only. | MapTiler free: 5k sessions + 100k requests/month, service pauses when exhausted [M1]. Stadia free: 200k credits/month, "commercial use not allowed" [S1, S2]. |
| Terms | Must show Google logo + data copyright; third-party data allowed only as overlays with separated attribution [G3]. Map Tiles API needs a session token [G4]. No pre-fetch, store or cache beyond Cache-Control; "offline uses" disallowed [G3]. MapLibre is not named as an allowed renderer; EEA billing addresses get different terms and reduced content since 2025-07-08 [G4]. | ODbL produced work: "© OpenStreetMap" attribution only [P1]. No key, no quotas, no caching limits. | MapTiler: "© MapTiler" logo on free plan + OSM attribution [M2]. Stadia: client cache max 7 days; offline only for mobile apps, 100 MB/device [S2]. Free tiers framed as non-commercial/evaluation; a free nonprofit app likely fits but needs written confirmation. |
| API key | Client-side; must be restricted by HTTP referrer / app ID in Cloud console (not verified in this spike). | None. | Client-side key (MapTiler) or domain auth (Stadia); key can be scraped and burn the quota. |
| Privacy | Google receives every visitor's IP, user agent, viewport/tile coordinates; JS API runs Google code in our page. Viewport around a lost-pet pin ≈ the visitor's neighbourhood. Conflicts with the spirit of ADR 0002; needs a consent/notice discussion. | Visitors only talk to our own host. Nothing leaves our infrastructure. Fits ADR 0002 fully. | Third party sees IP + viewport, like Google but without ad-network ties. Both claim GDPR compliance [M1, S1]; IP retention not verified. |
| Offline / SW | Not allowed (no pre-fetch, no offline) [G3]. | Allowed: SW can cache range responses or a low-zoom Serbia overview. | MapTiler: no clause found. Stadia: web offline not clearly allowed [S2]. |
| sr-Latn / sr-Cyrl | JS API takes `language=sr` (Cyrillic); Latin variant not verified. | Labels come from OSM `name:*`. In Serbia `name` is mostly Cyrillic and `name:sr-Latn` is widely tagged, but the Protomaps basemap's documented languages do not list `sr` [P4]. Likely needs a small build-time tweak or a client-side `coalesce(name:sr-Latn, name)` expression. Verify in follow-up. | MapTiler styles support `name:sr-Latn`/`sr` via its language switcher (not verified here). |
| Integration effort | JS API: rewrite map layer (markers, clustering, list sync) away from MapLibre. Map Tiles API in MapLibre: custom session-token handling + logo rules, unclear terms. High. | Lowest: pmtiles protocol already wired in `web/js/main.js`. Fix the source (currently `type: 'raster'`, must be `vector`) and add the Protomaps style, self-host glyphs + sprites (`protomaps/basemaps-assets`) [P1], vendor MapLibre + pmtiles into `web/vendor/`. | Low: swap style URL + key, add attribution. |
| Capacitor | JS API in a WebView works, but key restriction by referrer is awkward (`capacitor://localhost`); native SDKs need separate keys. | Same files load in the WebView; could even bundle a low-zoom extract in the app. | Works; keys exposed in the app bundle. |

## Recommendation

**(b) Self-hosted Protomaps for Serbia.** It is the only option that keeps visitors' locations off third-party servers, needs no billing account or key, allows offline caching, and reuses the MapLibre + pmtiles code already in story 004. Google is ruled out on privacy (viewport tracking), offline prohibition, billing-account risk, and rewrite cost. Keep MapTiler as the documented fallback if hosting egress becomes a problem, after confirming nonprofit eligibility in writing.

## Follow-up story (proposed: 009 self-hosted basemap)

1. Script (`make tiles`) that runs `pmtiles extract` on a pinned Protomaps daily build with Serbia bbox from `MAP.maxBounds`, `--maxzoom=15`; record file size.
2. Host the `.pmtiles` on our own origin (Range requests, CORS, long cache headers); set `MAP.pmtilesUrl`.
3. Switch the pmtiles source to `vector` and generate the style with `@protomaps/basemaps` (light flavor); labels sr-Latn / sr-Cyrl / en follow the app language toggle.
4. Self-host glyphs and sprites under `web/assets/`; vendor MapLibre GL and pmtiles JS under `web/vendor/` (pinned versions, checksums recorded); drop unpkg and demotiles.
5. Attribution "© OpenStreetMap contributors" + Protomaps link; remove OSM raster fallback from production config.
6. SW: cache style, glyphs, sprites; optional low-zoom tile cache with a size cap.
Acceptance: no request leaves our origin when the map loads (check in devtools); labels render in all three languages; CSP can drop third-party script hosts.

## Sources

- [G1] https://developers.google.com/maps/documentation/javascript/usage-and-billing
- [G2] https://developers.google.com/maps/billing-and-pricing/pricing
- [G3] https://developers.google.com/maps/documentation/tile/policies
- [G4] https://developers.google.com/maps/documentation/tile/2d-tiles-overview
- [P1] https://docs.protomaps.com/basemaps/downloads
- [P2] https://docs.protomaps.com/pmtiles/cli
- [P3] https://docs.protomaps.com/deploy/
- [P4] https://docs.protomaps.com/basemaps/localization
- [M1] https://www.maptiler.com/cloud/pricing/ · [M2] https://www.maptiler.com/terms/
- [S1] https://stadiamaps.com/pricing/ · [S2] https://stadiamaps.com/terms-of-service/

Not verified in this spike (pages were stubs or not fetched): Google Terms of Service and Service Specific Terms text, Google key-restriction docs, Google `sr-Latn` support, MapTiler/Stadia IP retention, Serbia extract exact size.
