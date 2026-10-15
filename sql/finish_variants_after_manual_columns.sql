-- For the database state shown in your phpMyAdmin screenshots:
-- cart_items has color and size; order_items has color and size;
-- uniq_user_product was already removed.
-- Run ONLY this statement in database dollie_shop.
-- Do not re-import product_variants.sql.
ALTER TABLE cart_items
  ADD UNIQUE KEY uniq_user_product_variant (user_id, product_id, color, size);
