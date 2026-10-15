-- Import once into the EXISTING dollie_shop database. Preserves products and orders.
CREATE TABLE IF NOT EXISTS categories (
 id BIGINT AUTO_INCREMENT PRIMARY KEY,
 name VARCHAR(80) NOT NULL,
 slug VARCHAR(100) NOT NULL UNIQUE,
 emoji VARCHAR(32) NOT NULL DEFAULT '🧸',
 image_url VARCHAR(500) NOT NULL DEFAULT '',
 sort_order INT NOT NULL DEFAULT 0
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
INSERT INTO categories(name,slug,emoji,sort_order) VALUES
('ตุ๊กตาหมี','bear','🧸',1),('ตุ๊กตากระต่าย','rabbit','🐰',2),('ตุ๊กตาลิง','monkey','🐵',3),('ตุ๊กตาหมู','pig','🐷',4),('ตุ๊กตาแมว','cat','🐱',5),('ตุ๊กตาสุนัข','dog','🐶',6),('ตุ๊กตาการ์ตูน-หนัง','cartoon','🎬',7)
ON DUPLICATE KEY UPDATE slug=VALUES(slug);
ALTER TABLE products ADD COLUMN category_id BIGINT NULL;
UPDATE products p SET category_id=(SELECT c.id FROM categories c WHERE c.slug = CASE
 WHEN LOWER(p.name) LIKE '%หมี%' OR LOWER(p.name) LIKE '%bear%' THEN 'bear'
 WHEN LOWER(p.name) LIKE '%กระต่าย%' OR LOWER(p.name) LIKE '%rabbit%' THEN 'rabbit'
 WHEN LOWER(p.name) LIKE '%ลิง%' THEN 'monkey'
 WHEN LOWER(p.name) LIKE '%หมู%' THEN 'pig'
 WHEN LOWER(p.name) LIKE '%แมว%' THEN 'cat'
 WHEN LOWER(p.name) LIKE '%สุนัข%' OR LOWER(p.name) LIKE '%หมา%' THEN 'dog'
 ELSE 'cartoon' END LIMIT 1) WHERE p.category_id IS NULL;
ALTER TABLE products ADD CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT;
