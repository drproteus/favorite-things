-- Tags move from separate tables into item descriptions as #hashtags.

-- Keep existing tags by appending them to each description. Spaces become underscores,
-- since a hashtag can't contain a space.
UPDATE items i
SET description = btrim(i.description || ' ' || sub.hashes)
FROM (
    SELECT it.item_id, string_agg('#' || replace(t.name, ' ', '_'), ' ' ORDER BY t.name) AS hashes
    FROM item_tags it
    JOIN tags t ON t.id = it.tag_id
    GROUP BY it.item_id
) sub
WHERE sub.item_id = i.id;

ALTER TABLE items ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

-- Same rule as the Go parser: a hashtag starts the text or follows a character that can't be part of a word.
UPDATE items i
SET tags = COALESCE(ARRAY(
    SELECT DISTINCT lower(m[2])
    FROM regexp_matches(i.description, '(^|[^[:alnum:]_&/])#([[:alnum:]_]+)', 'g') AS m
), '{}');

CREATE INDEX items_tags_idx ON items USING GIN (tags);

DROP TABLE item_tags;
DROP TABLE tags;
DROP TABLE site_tags;
