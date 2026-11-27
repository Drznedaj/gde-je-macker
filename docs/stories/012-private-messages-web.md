# Story 012: Optional contact, "Show contact" and message box (web)

Goal: Web side of ADR 0003. Posters can skip contact, public contact sits behind a click, and finders can message the owner privately.
Owner agent: frontend-dev
Files in scope: `web/**`
Out of scope: backend, sightings
Depends on: stories 004 and 005. Build against the mock API (`?mock=1`) until story 011 is merged.

## Acceptance criteria
- [ ] Post form (story 005): contact is optional. The hint explains that if they give it, it will be public, and that finders can always reach them through private messages.
- [ ] Post detail: public contact is behind a "Show contact" button that is reachable by keyboard. Posts without contact show no contact row.
- [ ] Every visible post has a "Message the owner" form: textarea (max 500), a character counter, and a note that the owner reads it privately and that they should not share anything they don't want the owner to have. Handle success, 422 and 429 with localized messages.
- [ ] Manage view (manage token from localStorage): list messages newest first, delete a message, report a message. Offer to turn on notifications for this post (Web Push permission requested only on click), and turn them off.
- [ ] If notifications are off, the manage view tells the owner to check back here for messages.
- [ ] Mock fixtures include posts with and without contact, and a manage view with messages.
- [ ] All new strings in sr-Latn, sr-Cyrl and en. Works at 360 px.

## Report back (under 200 words)
Summary, files touched, open questions.
