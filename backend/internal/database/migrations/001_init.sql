CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT,
    name        TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    is_admin    BOOLEAN NOT NULL DEFAULT false,
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

-- Folders. Each user has one root folder (parent_id IS NULL); every other folder nests under it.
CREATE TABLE collections (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id   UUID REFERENCES collections (id),
    name        TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    visibility  TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'private')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX collections_root_key ON collections (owner_id) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX collections_sibling_name_key ON collections (parent_id, lower(name)) WHERE parent_id IS NOT NULL;

CREATE TABLE items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    collection_id UUID NOT NULL REFERENCES collections (id),
    title         TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description   TEXT NOT NULL DEFAULT '',
    visibility    TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'private')),
    tags          TEXT[] NOT NULL DEFAULT '{}',
    rating        SMALLINT CHECK (rating BETWEEN 1 AND 5),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX items_owner_idx ON items (owner_id, updated_at DESC);
CREATE INDEX items_collection_idx ON items (collection_id, updated_at DESC);
CREATE INDEX items_public_idx ON items (updated_at DESC) WHERE visibility = 'public';
CREATE INDEX items_tags_idx ON items USING GIN (tags);

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
