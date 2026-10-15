package main

import (
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Payment struct {
	OrderID                  int64
	Customer, Method, Status string
	TotalCents               int64
	HasReceipt               bool
}

type DashboardStats struct {
	TotalOrders, PendingOrders, TotalProducts, LowStock, PendingPayments, UnansweredMessages int64
	CompletedSalesCents                                                                      int64
	RecentOrders                                                                             []DashboardOrder
}
type DashboardOrder struct {
	ID               int64
	Status, Customer string
	TotalCents       int64
}

func (a *App) adminMenu(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}
	if !method(w, r, "GET") || !a.adminAllowed(w, r) {
		return
	}
	a.page(w, r, "admin_menu.html", Page{Title: "แผงผู้ดูแล"})
}

func (a *App) admin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin/dashboard" {
		http.NotFound(w, r)
		return
	}
	if !method(w, r, "GET") || !a.adminAllowed(w, r) {
		return
	}
	var d DashboardStats
	queries := []struct {
		q    string
		dest *int64
	}{
		{`SELECT COUNT(*) FROM orders`, &d.TotalOrders},
		{`SELECT COUNT(*) FROM orders WHERE status='pending'`, &d.PendingOrders},
		{`SELECT COUNT(*) FROM products`, &d.TotalProducts},
		{`SELECT COUNT(*) FROM products WHERE stock<=5`, &d.LowStock},
		{`SELECT COALESCE(SUM(total_cents),0) FROM orders WHERE status='completed'`, &d.CompletedSalesCents},
		{`SELECT COUNT(*) FROM order_payments WHERE status='submitted'`, &d.PendingPayments},
		{`SELECT COUNT(*) FROM inquiries WHERE admin_reply IS NULL OR TRIM(admin_reply)=''`, &d.UnansweredMessages},
	}
	for _, q := range queries {
		if err := a.db.QueryRow(q.q).Scan(q.dest); err != nil {
			// Keep the dashboard available when an optional metric is not yet migrated.
			log.Printf("dashboard metric unavailable (%s): %v", q.q, err)
			*q.dest = 0
		}
	}
	rows, err := a.db.Query(`SELECT o.id,u.name,o.status,o.total_cents FROM orders o JOIN users u ON u.id=o.user_id ORDER BY o.id DESC LIMIT 5`)
	if err != nil {
		log.Printf("dashboard recent orders unavailable: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var v DashboardOrder
			if err = rows.Scan(&v.ID, &v.Customer, &v.Status, &v.TotalCents); err != nil {
				break
			}
			d.RecentOrders = append(d.RecentOrders, v)
		}
		if err == nil {
			err = rows.Err()
		}
		if err != nil {
			log.Printf("dashboard orders: %v", err)
			d.RecentOrders = nil
		}
	}
	a.page(w, r, "admin.html", Page{Title: "ภาพรวมร้านค้า", Dashboard: d})
}
func (a *App) adminCategories(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") || !a.adminAllowed(w, r) {
		return
	}
	a.page(w, r, "admin_categories.html", Page{Title: "จัดการหมวดหมู่", Categories: a.loadCategories()})
}
func (a *App) adminMessages(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") || !a.adminAllowed(w, r) {
		return
	}
	rows, e := a.db.Query(`SELECT i.id,u.name,i.subject,i.message,COALESCE(i.admin_reply,'') FROM inquiries i JOIN users u ON u.id=i.user_id ORDER BY i.id DESC LIMIT 200`)
	if e != nil {
		log.Print(e)
		http.Error(w, "database error (import sql/admin_messages_payments.sql)", 500)
		return
	}
	defer rows.Close()
	p := Page{Title: "ข้อความจากลูกค้า"}
	for rows.Next() {
		var v Inquiry
		if e = rows.Scan(&v.ID, &v.Name, &v.Subject, &v.Message, &v.Reply); e != nil {
			break
		}
		p.Inquiries = append(p.Inquiries, v)
	}
	if e == nil {
		e = rows.Err()
	}
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	a.page(w, r, "admin_messages.html", p)
}
func (a *App) replyInquiry(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}
	reply := strings.TrimSpace(r.FormValue("reply"))
	id := idFrom(r, "id")
	if id == 0 || len([]rune(reply)) < 1 || len([]rune(reply)) > 3000 {
		fail(w, "ข้อความไม่ถูกต้อง")
		return
	}
	res, e := a.db.Exec(`UPDATE inquiries SET admin_reply=?,replied_at=NOW() WHERE id=?`, reply, id)
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.NotFound(w, r)
		return
	}
	redirect(w, r, "/admin/messages")
}
func (a *App) adminPayments(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") || !a.adminAllowed(w, r) {
		return
	}
	rows, e := a.db.Query(`SELECT o.id,u.name,o.total_cents,p.method,p.status,(p.receipt_data IS NOT NULL) FROM order_payments p JOIN orders o ON o.id=p.order_id JOIN users u ON u.id=o.user_id ORDER BY o.id DESC LIMIT 200`)
	if e != nil {
		log.Print(e)
		http.Error(w, "database error (import sql/admin_messages_payments.sql)", 500)
		return
	}
	defer rows.Close()
	p := Page{Title: "การชำระเงิน"}
	for rows.Next() {
		var v Payment
		if e = rows.Scan(&v.OrderID, &v.Customer, &v.TotalCents, &v.Method, &v.Status, &v.HasReceipt); e != nil {
			break
		}
		p.Payments = append(p.Payments, v)
	}
	if e == nil {
		e = rows.Err()
	}
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	a.page(w, r, "admin_payments.html", p)
}
func (a *App) paymentStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}
	id := idFrom(r, "order_id")
	status := r.FormValue("status")
	if id == 0 || (status != "pending" && status != "submitted" && status != "verified" && status != "rejected") {
		fail(w, "ข้อมูลไม่ถูกต้อง")
		return
	}
	res, e := a.db.Exec(`UPDATE order_payments SET status=? WHERE order_id=?`, status, id)
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.NotFound(w, r)
		return
	}
	if r.FormValue("return_to") == "orders" {
		redirect(w, r, "/admin/orders")
		return
	}
	redirect(w, r, "/admin/payments")
}
func (a *App) uploadReceipt(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}
	if u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", 403)
		return
	}
	id := idFrom(r, "order_id")
	if id == 0 {
		fail(w, "คำสั่งซื้อไม่ถูกต้อง")
		return
	}
	f, _, e := r.FormFile("receipt")
	if e != nil {
		fail(w, "กรุณาเลือกไฟล์สลิป")
		return
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (3<<20)+1))
	if e != nil || len(data) == 0 || len(data) > 3<<20 {
		fail(w, "ไฟล์ต้องไม่เกิน 3 MB")
		return
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		fail(w, "รองรับเฉพาะ JPG PNG WEBP")
		return
	}
	res, e := a.db.Exec(`UPDATE order_payments p JOIN orders o ON o.id=p.order_id SET p.receipt_data=?,p.receipt_mime=?,p.status='submitted' WHERE p.order_id=? AND o.user_id=? AND p.method IN ('promptpay','mobile_banking') AND o.status<>'cancelled' AND p.status<>'verified'`, data, mime, id, u.ID)
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "ไม่สามารถส่งสลิปสำหรับคำสั่งซื้อนี้", 403)
		return
	}
	redirect(w, r, "/orders")
}
func (a *App) viewReceipt(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}
	id := idFrom(r, "order_id")
	var data []byte
	var mime string
	var e error
	if u.Role == "admin" {
		e = a.db.QueryRow(`SELECT receipt_data,receipt_mime FROM order_payments WHERE order_id=?`, id).Scan(&data, &mime)
	} else {
		e = a.db.QueryRow(`SELECT p.receipt_data,p.receipt_mime FROM order_payments p JOIN orders o ON o.id=p.order_id WHERE p.order_id=? AND o.user_id=?`, id, u.ID).Scan(&data, &mime)
	}
	if errors.Is(e, sql.ErrNoRows) || len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	if e != nil {
		log.Print(e)
		http.Error(w, "database error", 500)
		return
	}
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		http.Error(w, "invalid receipt", 500)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
func (a *App) loadCustomerMessages(w http.ResponseWriter, r *http.Request, u *User) ([]Inquiry, error) {
	rows, e := a.db.Query(`SELECT id,subject,message,COALESCE(admin_reply,'') FROM inquiries WHERE user_id=? ORDER BY id DESC LIMIT 100`, u.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var items []Inquiry
	for rows.Next() {
		var x Inquiry
		if e = rows.Scan(&x.ID, &x.Subject, &x.Message, &x.Reply); e != nil {
			return nil, e
		}
		items = append(items, x)
	}
	return items, rows.Err()
}
func paymentAccount() string { return os.Getenv("DOLLIE_PAYMENT_ACCOUNT") }
func paymentQR() string {
	if _, e := os.Stat("static/images/promptpay-qr.png"); e == nil {
		return "/static/images/promptpay-qr.png"
	}
	return ""
}
