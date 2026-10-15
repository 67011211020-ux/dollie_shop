Dollie Shop - ตะกร้าและหน้าสั่งซื้อแยกกัน

1. สำรองโฟลเดอร์เก่าไว้ก่อน
2. แตก ZIP แล้วเปิดโฟลเดอร์ dollie-shop ใน VS Code
3. เปิด XAMPP MySQL และ phpMyAdmin เลือกฐานข้อมูล dollie_shop
4. Import sql/checkout_shipping.sql เพียงครั้งเดียว (เพิ่มคอลัมน์ที่อยู่จัดส่งใน orders)
   ห้าม Import schema.sql หรือ cart_orders.sql ซ้ำ
5. ตั้ง MYSQL_DSN และ COOKIE_SECURE=false เหมือนเดิมใน PowerShell
6. go mod tidy แล้ว go run .
7. เข้าระบบเป็นลูกค้า เพิ่มตุ๊กตาอย่างน้อย 2 ตัว ไป /cart ติ๊ก 1 ตัว กดสั่งซื้อ
8. ไป /checkout กรอกชื่อ เบอร์โทร ที่อยู่ และยืนยัน จากนั้นตรวจ /orders
9. สินค้าที่ไม่ได้ติ๊กจะค้างในตะกร้า

คำเตือน: หากเคย Import checkout_shipping.sql ไปแล้ว อย่า Import ซ้ำ เพราะจะขึ้น Duplicate column.
โปรเจกต์นี้ไม่ตัดเงินจริง ไม่มีค่าจัดส่งหรือระบบชำระเงินจริง
