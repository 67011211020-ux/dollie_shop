-- Run once in phpMyAdmin > dollie_shop > SQL.
-- Existing products start with stock 0; set real quantities in Admin.
ALTER TABLE products ADD COLUMN stock INT NOT NULL DEFAULT 0;
