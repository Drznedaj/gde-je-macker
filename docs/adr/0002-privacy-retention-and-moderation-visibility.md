# ADR 0002: Privacy, retention and moderation visibility

Status: accepted (2026-11-27), based on a review of migration 0001.

1. **Public location is rounded.** The database keeps the exact last-seen point (needed for radius and alert-cell math). Public API responses round it to 3 decimals (about 100 m), because a last-seen spot is often the owner's home. Owners may later choose a different default if a real need appears.
2. **Held posts are hidden.** `held` (flagged by AI for human review) and `rejected` posts do not appear in listings or on the post page. `pending` and `approved` do. The push alert is sent only for `approved`.
3. **Retention.** A sweep job hard-deletes posts 14 days after they stop being active (expired, reunited or removed). This removes `contact`, photos, reports and moderation history with the post (foreign keys cascade). No moderation audit trail outlives its post, by design. Takedown evidence the operator must keep is a legal-backlog question (doc 00).
4. **Calibration data survives deletion** because `reunion_distances` holds only a distance and classes, with no link to any post.
5. **Contact is public by choice of the poster.** It is shown on the post, never stored in a user table, and removed by the retention sweep.
6. **Enums over magic values** for alert stage and moderation stage; length limits and range checks live in the schema and are mirrored in API validation.
