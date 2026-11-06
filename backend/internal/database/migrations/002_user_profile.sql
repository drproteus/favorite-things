ALTER TABLE users ADD COLUMN profile TEXT NOT NULL DEFAULT '' CHECK (char_length(profile) <= 2000);
ALTER TABLE users ADD COLUMN default_visibility TEXT NOT NULL DEFAULT 'public' CHECK (default_visibility IN ('public', 'private'));
