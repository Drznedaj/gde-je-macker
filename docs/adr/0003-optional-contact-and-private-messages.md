# ADR 0003: Optional public contact and private messages to the owner

Status: proposed (2026-11-27). Supersedes point 5 of ADR 0002 once accepted.

## Context
Under ADR 0002, contact is required and always public. Some owners don't want a phone number or email on a public page, where scrapers and scammers can harvest it. A finder still needs a way to reach them. Relaying messages to a hidden email or phone would mean storing contact details privately, which breaks the "no emails/phones in tables" rule.

## Decision
1. **Contact is optional.** If the poster gives it, it is public, as before (3-200 chars). If they leave it empty, the post shows no contact.
2. **Reveal on click.** The web client hides public contact behind a "Show contact" button. The API still returns it, so this is a deterrent against casual scraping, not a security boundary.
3. **Private message box on every post.** Anyone can leave a short text message on an active, visible post (max 500 chars, no photos, no location). Only the holder of the manage token can list or delete messages. We store no owner contact.
4. **Optional owner push.** From the manage view, the owner may register one push endpoint per post (FCM or Web Push) to be told "you have a new message". It is stored in its own table linked to the post, holds no message text, and is deleted with the post.
5. **Moderation.** Messages pass the same automated text check as posts before the owner can see them. Held messages stay hidden until a human approves them. The owner can delete any message or report it to moderation. Per-post and per-client rate limits run in memory; no IPs are stored.
6. **Retention.** Messages and the owner push endpoint cascade with the post, so the 14-day sweep in ADR 0002 removes them. The owner can delete a message sooner.
7. **Not sightings.** Messages carry no point, time or photo. Sightings remain out of the MVP and keep their own table.

## Consequences
- Migration: `posts.contact` becomes nullable, keeping the length check when present. New tables `post_messages` and `post_owner_push`.
- API: new public `POST /posts/{id}/messages`. Token-protected `GET /posts/{id}/messages`, `DELETE /posts/{id}/messages/{mid}`, and `PUT`/`DELETE /posts/{id}/owner-push`.
- A finder may type their own phone into a message. That is their choice, it is visible only to the owner, and it is deleted with the post.
- Owners without push have to open their manage link to see messages. The UI must say so.
