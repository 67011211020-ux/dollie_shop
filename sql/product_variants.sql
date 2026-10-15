-- Run once in phpMyAdmin, select database dollie_shop first.
-- Existing orders are preserved; old order items have NULL variants.
ALTER TABLE cart_items DROP INDEX uniq_user_product;
ALTER TABLE cart_items ADD COLUMN color VARCHAR(40) NOT NULL DEFAULT '' AFTER product_id, ADD COLUMN size VARCHAR(30) NOT NULL DEFAULT '' AFTER color;
ALTER TABLE cart_items ADD UNIQUE KEY uniq_user_product_variant (user_id, product_id, color, size);
ALTER TABLE order_items ADD COLUMN color VARCHAR(40) NULL AFTER product_id, ADD COLUMN size VARCHAR(30) NULL AFTER color;
