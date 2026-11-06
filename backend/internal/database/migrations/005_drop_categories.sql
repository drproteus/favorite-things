-- Categories are removed for now. Organization will come from collections instead.
DROP INDEX IF EXISTS items_category_idx;
ALTER TABLE items DROP COLUMN IF EXISTS category_id;
DROP TABLE IF EXISTS site_categories;
