CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT,
    name        TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE oauth_accounts (
    provider         TEXT NOT NULL CHECK (provider IN ('google', 'github')),
    provider_user_id TEXT NOT NULL,
    user_id          UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_user_id)
);
CREATE INDEX oauth_accounts_user_idx ON oauth_accounts (user_id);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT '',
    visibility  TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'private')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX items_owner_idx ON items (owner_id, updated_at DESC);
CREATE INDEX items_public_idx ON items (updated_at DESC) WHERE visibility = 'public';

CREATE TABLE tags (
    id      BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name    TEXT NOT NULL,
    UNIQUE (user_id, name)
);

CREATE TABLE item_tags (
    item_id UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    tag_id  BIGINT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);
CREATE INDEX item_tags_tag_idx ON item_tags (tag_id);

CREATE TABLE item_images (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id      UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    s3_key       TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    position     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX item_images_item_idx ON item_images (item_id, position);

CREATE TABLE item_ratings (
    item_id    UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rating     SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (item_id, user_id)
);
