-- Resized variants for item photos and folder banners. Width/height are the
-- original dimensions; variants is a JSON array of the generated objects.
ALTER TABLE item_images
    ADD COLUMN width INT NOT NULL DEFAULT 0,
    ADD COLUMN height INT NOT NULL DEFAULT 0,
    ADD COLUMN variants JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE collections
    ADD COLUMN banner_width INT NOT NULL DEFAULT 0,
    ADD COLUMN banner_height INT NOT NULL DEFAULT 0,
    ADD COLUMN banner_variants JSONB NOT NULL DEFAULT '[]'::jsonb;
