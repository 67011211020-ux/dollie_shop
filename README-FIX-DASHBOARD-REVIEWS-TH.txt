Dollie Shop - Dashboard + Verified Reviews + 9 Product Colors

1. Open PowerShell inside dollie-shop (the folder with go.mod).
2. Start XAMPP MySQL, set MYSQL_DSN and COOKIE_SECURE, then run: go run .
3. Admin dashboard: if an optional metric table/column is missing, that metric displays 0 instead of HTTP 500.
   IMPORTANT: This does NOT create missing MySQL tables or data. Read PowerShell logs
   starting with "dashboard metric unavailable" and import the required SQL migration.
4. Reviews: existing logic preserved. Only confirmed/completed purchases allow reviews;
   one review per qualifying order. Everyone can see reviews.
5. Colors: only first option replaced for product IDs 6,7,24,25,21,20,19,18,10.
   Remaining color options unchanged.

Never import sql/schema.sql into an existing database without a backup.
