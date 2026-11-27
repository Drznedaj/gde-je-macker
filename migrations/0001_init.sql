-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TYPE post_kind AS ENUM ('lost', 'found');
CREATE TYPE species AS ENUM ('dog', 'cat');
CREATE TYPE pet_sex AS ENUM ('male', 'female', 'unknown');
CREATE TYPE tri_state AS ENUM ('yes', 'no', 'unknown');
CREATE TYPE post_status AS ENUM ('active', 'reunited', 'expired', 'removed');
CREATE TYPE moderation_state AS ENUM ('pending', 'approved', 'held', 'rejected');
CREATE TYPE density_class AS ENUM ('urban', 'suburban', 'rural');

CREATE TABLE posts (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind            post_kind NOT NULL,
  species         species NOT NULL,
  sex             pet_sex NOT NULL DEFAULT 'unknown',
  neutered        tri_state NOT NULL DEFAULT 'unknown',
  goes_outside    tri_state NOT NULL DEFAULT 'unknown',
  name            text,
  description     text NOT NULL DEFAULT '',
  last_seen_point geography(Point, 4326) NOT NULL,
  last_seen_at    timestamptz NOT NULL,
  contact         text NOT NULL,              -- poster-provided, shown on the post
  density         density_class,
  radius_m        integer,                    -- computed from config/radius.yaml
  status          post_status NOT NULL DEFAULT 'active',
  moderation      moderation_state NOT NULL DEFAULT 'pending',
  alert_stage     smallint NOT NULL DEFAULT 0, -- 0 none, 1 sent, 2 widened
  manage_token_hash bytea NOT NULL,           -- hash only; token lives in the owner's secret link
  created_at      timestamptz NOT NULL DEFAULT now(),
  expires_at      timestamptz NOT NULL
);
CREATE INDEX posts_point_gix ON posts USING gist (last_seen_point);
CREATE INDEX posts_active_idx ON posts (status, moderation, created_at DESC);

CREATE TABLE post_photos (
  id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id  uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  path     text NOT NULL,
  width    integer NOT NULL,
  height   integer NOT NULL,
  position smallint NOT NULL DEFAULT 0
);

CREATE TABLE reports (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  reason     text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE moderation_log (
  id         bigserial PRIMARY KEY,
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  stage      text NOT NULL,                   -- deterministic | omni | llm | human
  verdict    moderation_state NOT NULL,
  reason     text,
  model      text,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Push endpoints/tokens plus coarse geohash cells only. No exact location, no identity.
CREATE TABLE push_subscriptions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transport  text NOT NULL CHECK (transport IN ('fcm', 'webpush')),
  endpoint   text NOT NULL UNIQUE,            -- FCM token or Web Push endpoint
  p256dh     text,
  auth       text,
  cells      text[] NOT NULL,                 -- geohash cells (precision 5 or 6)
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_cells_gin ON push_subscriptions USING gin (cells);

-- Calibration data: distance and class only, never the point.
CREATE TABLE reunion_distances (
  id           bigserial PRIMARY KEY,
  species      species NOT NULL,
  goes_outside tri_state NOT NULL,
  sex          pet_sex NOT NULL,
  neutered     tri_state NOT NULL,
  density      density_class,
  meters       integer NOT NULL CHECK (meters >= 0),
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- Planned for later (sightings feature): table exists, no API or UI in the MVP.
CREATE TABLE sightings (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id    uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  point      geography(Point, 4326) NOT NULL,
  seen_at    timestamptz NOT NULL,
  note       text NOT NULL DEFAULT '',
  photo_path text,
  moderation moderation_state NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sightings_post_idx ON sightings (post_id, seen_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sightings, reunion_distances, push_subscriptions, moderation_log, reports, post_photos, posts;
DROP TYPE IF EXISTS density_class, moderation_state, post_status, tri_state, pet_sex, species, post_kind;
