แก้หน้า /admin/orders: รองรับ order_items.product_id ที่เป็น NULL สำหรับรายการสินค้าเก่า โดยแสดงชื่อ สินค้าที่ถูกลบ แทนการเกิด database error และเพิ่ม log ใน PowerShell เมื่อเกิดข้อผิดพลาดจริง
ไม่ลบหรือแก้ข้อมูล MySQL และคงเงื่อนไขรีวิวเดิมไว้
เปิด PowerShell ภายในโฟลเดอร์ dollie-shop (ที่มี go.mod) แล้วรัน go run .
