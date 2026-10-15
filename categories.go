package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) loadCategories() []Category {
	rows, err := a.db.Query(`SELECT id,name,slug,emoji,image_url FROM categories ORDER BY sort_order,id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Emoji, &c.ImageURL) == nil {
			out = append(out, c)
		}
	}
	return out
}
func (a *App) saveCategory(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}
	id := idFrom(r, "id")
	name := strings.TrimSpace(r.FormValue("name"))
	emoji := strings.TrimSpace(r.FormValue("emoji"))
	imageURL := strings.TrimSpace(r.FormValue("image_url"))
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 || len([]rune(emoji)) > 8 || len(imageURL) > 500 {
		fail(w, "ข้อมูลหมวดหมู่ไม่ถูกต้อง")
		return
	}
	if imageURL != "" && !strings.HasPrefix(imageURL, "/static/category-images/") {
		u, e := url.Parse(imageURL)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			fail(w, "รูปภาพ URL ต้องเป็น https://")
			return
		}
	}
	file, header, err := r.FormFile("image_file")
	if err == nil {
		defer file.Close()
		if header.Size > 3<<20 {
			fail(w, "รูปภาพต้องไม่เกิน 3 MB")
			return
		}
		first := make([]byte, 512)
		n, _ := io.ReadFull(file, first)
		contentType := http.DetectContentType(first[:n])
		ext := ""
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		default:
			fail(w, "รองรับ JPG PNG WEBP เท่านั้น")
			return
		}
		if _, err = file.Seek(0, 0); err != nil {
			fail(w, "อ่านไฟล์รูปไม่สำเร็จ")
			return
		}
		token, e := randomToken()
		if e != nil {
			http.Error(w, "upload error", 500)
			return
		}
		dir := filepath.Join("static", "category-images")
		if e = os.MkdirAll(dir, 0755); e != nil {
			http.Error(w, "upload error", 500)
			return
		}
		dest, e := os.OpenFile(filepath.Join(dir, token+ext), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e != nil {
			http.Error(w, "upload error", 500)
			return
		}
		_, e = io.Copy(dest, io.LimitReader(file, 3<<20+1))
		ce := dest.Close()
		if e != nil || ce != nil {
			http.Error(w, "upload error", 500)
			return
		}
		imageURL = "/static/category-images/" + token + ext
	} else if err != http.ErrMissingFile {
		fail(w, "ไฟล์รูปภาพไม่ถูกต้อง")
		return
	}
	if id == 0 {
		slug, err := randomToken()
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		_, err = a.db.Exec(`INSERT INTO categories(name,slug,emoji,image_url) VALUES(?,?,?,?)`, name, "category-"+slug[:16], emoji, imageURL)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
	} else {
		var old string
		err := a.db.QueryRow(`SELECT image_url FROM categories WHERE id=?`, id).Scan(&old)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		if imageURL == "" && r.FormValue("remove_image") != "1" {
			imageURL = old
		}
		_, err = a.db.Exec(`UPDATE categories SET name=?,emoji=?,image_url=? WHERE id=?`, name, emoji, imageURL, id)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
	}
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}
func (a *App) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}
	id := idFrom(r, "id")
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM products WHERE category_id=?`, id).Scan(&count); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if count > 0 {
		http.Error(w, fmt.Sprintf("หมวดหมู่นี้มีสินค้า %d รายการ กรุณาย้ายสินค้าไปหมวดอื่นก่อนลบ", count), 400)
		return
	}
	if _, err := a.db.Exec(`DELETE FROM categories WHERE id=?`, id); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}
