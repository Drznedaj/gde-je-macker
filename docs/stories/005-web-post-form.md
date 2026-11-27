# Story 005: Web post form (lost/found) with photo upload

Goal: form to create a lost or found post with map pin picking, up to 3 photos and clear validation, against the mock API.
Owner agent: frontend-dev
Files in scope: web/**
Out of scope: backend, alerts, accounts
Depends on: story 004 (map, i18n, api.js)

## Acceptance criteria
- [ ] Fields per openapi.yaml: kind, species, name, description, sex, neutered, goes_outside, last-seen point and time, contact, photos (max 3).
- [ ] Pin picker on the map; point restricted to Serbia client-side (server still validates).
- [ ] Client-side image downscale before upload; show preview; remove/reorder.
- [ ] Validation mirrors server limits (name 80, description 2000, contact 3-200) with localized messages.
- [ ] After success, the manage token is shown once with a clear "save this link" instruction and stored only in localStorage on the device.
- [ ] Edit, renew and mark-reunited flows use the manage token.
- [ ] Posters are told the contact is public. If ADR 0003 is accepted before this story starts, contact is optional and story 012 adds the hint and message box; otherwise keep it required.

## Report back (under 200 words)
Summary, files touched, open questions.
