-- Ordering for admin-managed lists.
ALTER TABLE site_categories ADD COLUMN sort_order INT NOT NULL DEFAULT 0;
ALTER TABLE site_tags ADD COLUMN sort_order INT NOT NULL DEFAULT 0;

UPDATE site_categories c SET sort_order = o.rn
FROM (SELECT id, row_number() OVER (ORDER BY lower(name)) AS rn FROM site_categories) o
WHERE c.id = o.id;

UPDATE site_tags t SET sort_order = o.rn
FROM (SELECT id, row_number() OVER (ORDER BY lower(name)) AS rn FROM site_tags) o
WHERE t.id = o.id;

-- Items reference a fixed category instead of free text. Existing text is matched by name;
-- anything that doesn't match stays uncategorized until the owner picks one.
ALTER TABLE items ADD COLUMN category_id BIGINT REFERENCES site_categories (id) ON DELETE RESTRICT;
UPDATE items i SET category_id = c.id
FROM site_categories c
WHERE lower(c.name) = lower(i.category);
ALTER TABLE items DROP COLUMN category;
CREATE INDEX items_category_idx ON items (category_id, updated_at DESC);

-- Global ratings go away. A rating is now a single value the owner sets on their own item.
-- Keep the owner's existing self-ratings; drop the rest of the ratings table.
ALTER TABLE items ADD COLUMN rating SMALLINT CHECK (rating BETWEEN 1 AND 5);
UPDATE items i SET rating = r.rating
FROM item_ratings r
WHERE r.item_id = i.id AND r.user_id = i.owner_id;
DROP TABLE item_ratings;
