# Story 002: Radius calculation

Goal: Go function that turns a post's species, goes_outside, sex, neutered and density class into stage 1 and stage 2 radii from config/radius.yaml.
Owner agent: backend-dev (reads config), data-radius (owns the config values)
Files in scope: server/internal/radius/**, config/radius.yaml (read only for backend-dev)
Out of scope: geohash cell publishing, push

## Acceptance criteria
- [ ] Table-driven tests cover each base case and multiplier.
- [ ] Result clamped to the min and max in the config.
- [ ] Unknown values fall back to the conservative (larger-uncertainty) base case, documented in code comments.
