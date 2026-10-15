-- Run once for existing installations (new installs use schema.sql).
ALTER TABLE reviews ADD COLUMN deleted_at DATETIME NULL DEFAULT NULL;
