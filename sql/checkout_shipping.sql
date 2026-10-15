-- Run ONCE in phpMyAdmin > dollie_shop > SQL. Do not re-import schema.sql.
USE dollie_shop;
ALTER TABLE orders
  ADD COLUMN receiver_name VARCHAR(120) NULL,
  ADD COLUMN receiver_phone VARCHAR(20) NULL,
  ADD COLUMN shipping_address TEXT NULL,
  ADD COLUMN order_note VARCHAR(500) NULL;
