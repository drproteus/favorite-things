CREATE TABLE item_comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id    UUID NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    author_id  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body       TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    -- The API currently only creates "signed_in" comments, but the schema already
    -- allows per-comment visibility to be expanded without another migration.
    visibility TEXT NOT NULL DEFAULT 'signed_in' CHECK (visibility IN ('signed_in', 'public', 'private')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX item_comments_item_idx ON item_comments (item_id, created_at);
CREATE INDEX item_comments_author_idx ON item_comments (author_id);
