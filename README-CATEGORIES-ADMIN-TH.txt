Dollie Shop - จัดการหมวดหมู่โดยแอดมิน

1. สำรองฐานข้อมูล dollie_shop ก่อน
2. เปิด phpMyAdmin เลือกฐานข้อมูล dollie_shop > Import > sql/categories_admin.sql (เพียงครั้งเดียว)
3. เปิด VS Code ในโฟลเดอร์ dollie-shop ที่มี go.mod
4. ตั้ง MYSQL_DSN และ COOKIE_SECURE ตามเครื่องเดิม แล้ว go run .
5. เข้า /admin > จัดการหมวดหมู่ตุ๊กตา
6. เพิ่ม แก้ไข ลบหมวดหมู่ และอัปโหลดรูป JPG/PNG/WEBP ไม่เกิน 3MB หรือใส่ HTTPS URL
7. เวลาเพิ่มหรือแก้ไขสินค้า ต้องเลือกหมวดหมู่จากรายการ

หมายเหตุ:
- ห้าม Import sql/schema.sql ใหม่
- ถ้ามีสินค้าในหมวดหมู่ จะไม่สามารถลบหมวดหมู่นั้นจนกว่าจะย้ายสินค้า
- รูปที่อัปโหลดเก็บไว้ใน static/category-images/ ต้องเก็บโฟลเดอร์นี้ไว้เมื่อย้ายโปรเจกต์
- สินค้าเดิมถูกจัดหมวดหมู่จากชื่อสินค้าโดยประมาณ ตรวจสอบและแก้ไขใน Admin ได้
- ถ้าเคยเพิ่ม category_id ในตาราง products แล้ว ให้ข้ามบรรทัด ALTER TABLE products ADD COLUMN category_id BIGINT NULL; ใน SQL
