# Dollie Shop: Render Free + Aiven MySQL

1. Create Aiven for MySQL Free at https://console.aiven.io/ and record host, port, user, password and TLS instructions.
2. Create database `dollie_shop`. Import `sql/schema.sql`, then required SQL migrations from `sql/` in order. Do not run ALTER twice on existing databases.
3. Connect using Go MySQL DSN configured with TLS (e.g. `tls=true` only if certificate validation succeeds). Never use `tls=skip-verify`.
4. Push the `dollie-shop` directory to GitHub, excluding `.env`, passwords, customer data, and local uploads.
5. Render > New > Web Service > GitHub repo > Docker > Free. Set Root Directory to the folder containing `Dockerfile`.
6. Environment: `MYSQL_DSN` = Aiven connection string; `COOKIE_SECURE=true`; `PORT=10000`.
7. Deploy and test User A, User B, Admin, login, review CRUD, checkout and admin forms.
8. Render Free local uploads disappear on restart/spin-down/redeploy; category images need persistent object storage for lasting uploads.
9. Rotate previously disclosed local MySQL passwords before publishing the repository.

Docs: https://render.com/docs/free ; https://aiven.io/docs/platform/concepts/service-pricing
