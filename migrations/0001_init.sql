-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TYPE post_kind AS ENUM ('lost', 'found');
CREATE TYPE species AS ENUM ('dog', 'cat');
CREATE TYPE pet_sex AS ENUM ('male', 'female', 'unknown');
CREATE TYPE tri_state AS ENUM ('yes', 'no', 'unknown');
CREATE TYPE post_status AS ENUM ('active', 'reunited', 'expired', 'removed');
-- held = flagged by AI for human review; hidden from public listings like rejected.
CREATE TYPE moderation_state AS ENUM ('pending', 'approved', 'held', 'rejected');
CREATE TYPE moderation_stage AS ENUM ('deterministic', 'omni', 'llm', 'human');
CREATE TYPE alert_stage AS ENUM ('none', 'initial', 'widened');
CREATE TYPE density_class AS ENUM ('urban', 'suburban', 'rural');

CREATE TABLE posts (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind            post_kind NOT NULL,
  species         species NOT NULL,
  sex             pet_sex NOT NULL DEFAULT 'unknown',
  neutered        tri_state NOT NULL DEFAULT 'unknown',
  goes_outside    tri_state NOT NULL DEFAULT 'unknown',
  name            text CHECK (char_length(name) <= 80),
  description     text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
  last_seen_point geography(Point, 4326) NOT NULL,  -- exact; the public API serves it rounded (see ADR 0002)
  last_seen_at    timestamptz NOT NULL,
  contact         text NOT NULL CHECK (char_length(contact) BETWEEN 3 AND 200), -- poster-provided, public
  density         density_class,
  radius_m        integer CHECK (radius_m > 0),     -- computed from config/radius.yaml
  status          post_status NOT NULL DEFAULT 'active',
  moderation      moderation_state NOT NULL DEFAULT 'pending',
  alert_stage     alert_stage NOT NULL DEFAULT 'none',
  manage_token_hash bytea NOT NULL,                 -- hash only; token lives in the owner's secret link
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  reunited_at     timestamptz,
  expires_at      timestamptz NOT NULL,
  CHECK (expires_at > created_at)
);
-- Public listings and map: only active posts that are pending or approved (not held/rejected).
-- Queries MUST repeat this predicate to use these partial indexes.
CREATE INDEX posts_public_point_gix ON posts USING gist (last_seen_point)
  WHERE status = 'active' AND moderation IN ('pending', 'approved');
CREATE INDEX posts_public_recent_idx ON posts (created_at DESC)
  WHERE status = 'active' AND moderation IN ('pending', 'approved');
CREATE INDEX posts_expiry_idx ON posts (expires_at) WHERE status = 'active';
CREATE INDEX posts_cleanup_idx ON posts (expires_at) WHERE status <> 'active';  -- retention sweep
CREATE INDEX posts_pending_idx ON posts (created_at) WHERE moderation = 'pending';

CREATE TABLE post_photos (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id  uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  path     text NOT NULL CHECK (char_length(path) <= 300),
  width    integer NOT NULL CHECK (width > 0),
  height   integer NOT NULL CHECK (height > 0),
  position smallint NOT NULL CHECK (position BETWEEN 0 AND 2),  -- max 3 photos per post
  UNIQUE (post_id, position)                                    -- also serves as the post_id index
);

CREATE TABLE reports (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  reason     text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  handled_at timestamptz
);
CREATE INDEX reports_post_idx ON reports (post_id);
CREATE INDEX reports_unhandled_idx ON reports (created_at) WHERE handled_at IS NULL;

-- Cascades with the post on purpose: no moderation history outlives a deleted post (ADR 0002).
CREATE TABLE moderation_log (
  id         bigserial PRIMARY KEY,
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  stage      moderation_stage NOT NULL,
  verdict    moderation_state NOT NULL,
  reason     text CHECK (char_length(reason) <= 1000),
  model      text CHECK (char_length(model) <= 100),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX moderation_log_post_idx ON moderation_log (post_id, created_at);

-- Push endpoints/tokens plus coarse geohash cells only. No exact location, no identity.
CREATE TABLE push_subscriptions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transport  text NOT NULL CHECK (transport IN ('fcm', 'webpush')),
  endpoint   text NOT NULL UNIQUE CHECK (char_length(endpoint) <= 2048),  -- FCM token or Web Push endpoint
  p256dh     text,
  auth       text,
  cells      text[] NOT NULL CHECK (cardinality(cells) BETWEEN 1 AND 200), -- geohash, precision 5 or 6
  lang       text NOT NULL DEFAULT 'sr-Latn' CHECK (lang IN ('sr-Latn', 'sr-Cyrl', 'en')),
  species_filter species[],                 -- NULL = all species
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen  timestamptz NOT NULL DEFAULT now(),
  CHECK (transport <> 'webpush' OR (p256dh IS NOT NULL AND auth IS NOT NULL))
);
CREATE INDEX push_cells_gin ON push_subscriptions USING gin (cells);

-- Calibration data: distance and class only, never the point and no link to a post.
CREATE TABLE reunion_distances (
  id             bigserial PRIMARY KEY,
  kind           post_kind NOT NULL,
  species        species NOT NULL,
  goes_outside   tri_state NOT NULL,
  sex            pet_sex NOT NULL,
  neutered       tri_state NOT NULL,
  density        density_class,
  meters         integer NOT NULL CHECK (meters BETWEEN 0 AND 500000),
  days_to_reunion integer CHECK (days_to_reunion >= 0),
  created_at     timestamptz NOT NULL DEFAULT now()
);

-- Planned for later (sightings feature): table exists, no API or UI in the MVP.
CREATE TABLE sightings (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  point      geography(Point, 4326) NOT NULL,
  seen_at    timestamptz NOT NULL,
  note       text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  photo_path text CHECK (char_length(photo_path) <= 300),
  moderation moderation_state NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sightings_post_idx ON sightings (post_id, seen_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sightings, reunion_distances, push_subscriptions, moderation_log, reports, post_photos, posts;
DROP TYPE IF EXISTS density_class, alert_stage, moderation_stage, moderation_state, post_status, tri_state, pet_sex, species, post_kind;
