-- Filename: migrations/000003_create_review_table.up.sql
CREATE TABLE IF NOT EXISTS review (
    rid bigserial PRIMARY KEY,
    prod_id INTEGER NOT NULL,
    created_at timestamp(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    rating int NOT NULL,
    helpful_count int DEFAULT 0,
    CONSTRAINT fk_product
    FOREIGN KEY (prod_id)
    REFERENCES product (pid)
    ON UPDATE CASCADE
    ON DELETE CASCADE
);