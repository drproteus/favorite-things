ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE site_categories (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX site_categories_name_key ON site_categories (lower(name));

CREATE TABLE site_tags (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX site_tags_name_key ON site_tags (lower(name));

INSERT INTO site_categories (name) VALUES
    ('Albums'), ('Vinyl'), ('Books'), ('Comics'), ('Board Games'), ('Cameras'),
    ('Consoles'), ('Video Games'), ('Movies'), ('Posters'), ('Sneakers'),
    ('T-Shirts'), ('Toys'), ('Trading Cards'), ('Watches');

INSERT INTO site_tags (name) VALUES
    ('favorite'), ('wishlist'), ('rare'), ('limited edition'), ('signed'),
    ('mint condition'), ('needs repair'), ('gift'), ('vintage'), ('collectible');
