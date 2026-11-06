ALTER TABLE collections ADD COLUMN description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000);
ALTER TABLE collections ADD COLUMN banner_key TEXT;
