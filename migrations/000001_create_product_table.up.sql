-- Filename: migrations/000002_create_product_table.up.sql
CREATE TABLE IF NOT EXISTS product (
    pid bigserial PRIMARY KEY,
    created_at timestamp(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    pname text NOT NULL,
    product_category product_category NOT NULL,
    image_URL text NOT NULL,
    avg_rating DECIMAL(3,2)
);