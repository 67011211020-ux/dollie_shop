-- Run once in phpMyAdmin SQL tab, database dollie_shop, AFTER existing migrations.
USE dollie_shop;
ALTER TABLE inquiries ADD COLUMN admin_reply TEXT NULL, ADD COLUMN replied_at DATETIME NULL;
CREATE TABLE IF NOT EXISTS order_payments (
 order_id BIGINT PRIMARY KEY,
 method ENUM('promptpay','mobile_banking','cod') NOT NULL,
 status ENUM('pending','submitted','verified','rejected') NOT NULL DEFAULT 'pending',
 receipt_data MEDIUMBLOB NULL,
 receipt_mime VARCHAR(30) NULL,
 created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
) ENGINE=InnoDB;
