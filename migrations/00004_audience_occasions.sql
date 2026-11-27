-- +goose Up
ALTER TABLE products
    ADD COLUMN audience  VARCHAR(10),
    ADD COLUMN occasions JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE products ADD CONSTRAINT products_audience_chk CHECK (audience IN ('pria', 'wanita', 'unisex'));
ALTER TABLE products ADD CONSTRAINT products_occasions_chk CHECK (
    jsonb_typeof(occasions) = 'array'
    AND occasions <@ '["harian", "kantor", "malam", "acara", "santai", "olahraga", "ibadah"]'::jsonb
);
ALTER TABLE products ADD CONSTRAINT products_general_audience_chk CHECK (
    kind = 'perfume' OR (audience IS NULL AND occasions = '[]'::jsonb)
);

-- +goose Down
ALTER TABLE products DROP CONSTRAINT products_general_audience_chk;
ALTER TABLE products DROP CONSTRAINT products_occasions_chk;
ALTER TABLE products DROP CONSTRAINT products_audience_chk;
ALTER TABLE products DROP COLUMN occasions, DROP COLUMN audience;
