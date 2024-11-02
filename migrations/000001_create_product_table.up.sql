-- Filename: migrations/000001_create_product_table.up.sql
CREATE TABLE IF NOT EXISTS product (
    pid bigserial PRIMARY KEY,
    created_at timestamp(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    pname text NOT NULL,
    product_category text,
    image_URL text,
    avg_rating DECIMAL(3,2)
);