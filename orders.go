package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"strconv"
	"strings"
)

func money(c int64) string {
	return strconv.FormatInt(c/100, 10) + "." + func() string {
		x := strconv.FormatInt(c%100, 10)
		if len(x) == 1 {
			return "0" + x
		}
		return x
	}()
}
func (a *App) postUser(w http.ResponseWriter, r *http.Request) *User {
	if !method(w, r, "POST") {
		return nil
	}
	u := a.currentUser(r)
	if !required(w, u) {
		return nil
	}
	if !a.validCSRF(r) {
		http.Error(w, "CSRF ไม่ถูกต้อง", 403)
		return nil
	}
	return u
}
func validChoice(v string, choices []string) bool {
	for _, x := range choices {
		if v == x {
			return true
		}
	}
	return false
}
func (a *App) cartAdd(w http.ResponseWriter, r *http.Request) {
	u := a.postUser(w, r)
	if u == nil {
		return
	}
	if u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", http.StatusForbidden)
		return
	}
	id := idFrom(r, "product_id")
	qty, qtyErr := strconv.Atoi(r.FormValue("quantity"))
	if qtyErr != nil || qty < 1 || qty > 99 {
		fail(w, "จำนวนต้องอยู่ระหว่าง 1-99")
		return
	}
	color, size := strings.TrimSpace(r.FormValue("color")), strings.TrimSpace(r.FormValue("size"))
	validColor, validSize := false, false
	for _, v := range productColors(id) {
		if color == v {
			validColor = true
			break
		}
	}
	for _, v := range []string{"1.0 เมตร", "1.2 เมตร", "1.4 เมตร", "1.6 เมตร", "1.8 เมตร", "2.0 เมตร"} {
		if size == v {
			validSize = true
			break
		}
	}
	if !validColor || !validSize {
		fail(w, "กรุณาเลือกสีและขนาดให้ถูกต้อง")
		return
	}
	if id < 1 {
		fail(w, "สินค้าไม่ถูกต้อง")
		return
	}
	res, e := a.db.Exec(`INSERT INTO cart_items (user_id,product_id,color,size,quantity) SELECT ?,id,?,?,? FROM products WHERE id=? ON DUPLICATE KEY UPDATE quantity=LEAST(quantity+VALUES(quantity),99)`, u.ID, color, size, qty, id)
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
	redirect(w, r, "/cart")
}
func (a *App) cartUpdate(w http.ResponseWriter, r *http.Request) {
	u := a.postUser(w, r)
	if u == nil {
		return
	}
	if u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", http.StatusForbidden)
		return
	}
	id := idFrom(r, "id")
	qty, e := strconv.Atoi(r.FormValue("quantity"))
	if id < 1 || e != nil || qty < 0 || qty > 99 {
		fail(w, "จำนวนไม่ถูกต้อง (0-99)")
		return
	}
	if qty == 0 {
		_, e = a.db.Exec(`DELETE FROM cart_items WHERE id=? AND user_id=?`, id, u.ID)
	} else {
		_, e = a.db.Exec(`UPDATE cart_items SET quantity=? WHERE id=? AND user_id=?`, qty, id, u.ID)
	}
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}
	redirect(w, r, "/cart")
}
func (a *App) cart(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}
	if u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", http.StatusForbidden)
		return
	}
	p := Page{Title: "ตะกร้าสินค้า"}
	rows, e := a.db.Query(`SELECT c.id,c.product_id,p.name,p.image_url,c.color,c.size,c.quantity,CAST(ROUND(p.price*100) AS SIGNED) FROM cart_items c JOIN products p ON p.id=c.product_id WHERE c.user_id=? ORDER BY c.id`, u.ID)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x CartItem
		if e = rows.Scan(&x.ID, &x.ProductID, &x.Name, &x.ImageURL, &x.Color, &x.Size, &x.Quantity, &x.PriceCents); e != nil {
			break
		}
		x.SubtotalCents = x.PriceCents * int64(x.Quantity)
		p.TotalCents += x.SubtotalCents
		p.CartItems = append(p.CartItems, x)
	}
	if e == nil {
		e = rows.Err()
	}
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}
	a.page(w, r, "cart.html", p)
}
func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}
	if u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", 403)
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.Method == "POST" && !a.validCSRF(r) {
		http.Error(w, "CSRF ไม่ถูกต้อง", 403)
		return
	}
	// GET previews the chosen items; POST creates the order only after confirmation.
	if err := r.ParseForm(); err != nil {
		fail(w, "ข้อมูลไม่ถูกต้อง")
		return
	}
	selected := r.Form["selected_ids"]
	if len(selected) == 0 || len(selected) > 99 {
		fail(w, "กรุณาเลือกสินค้าที่ต้องการสั่งซื้อ")
		return
	}
	ids := make(map[int64]bool)
	for _, raw := range selected {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 || ids[id] {
			fail(w, "รายการสินค้าที่เลือกไม่ถูกต้อง")
			return
		}
		ids[id] = true
	}
	if r.Method == "GET" {
		rows, err := a.db.Query(`SELECT c.id,c.product_id,p.name,p.image_url,c.color,c.size,c.quantity,CAST(ROUND(p.price*100) AS SIGNED) FROM cart_items c JOIN products p ON p.id=c.product_id WHERE c.user_id=? ORDER BY c.id`, u.ID)
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		page := Page{Title: "ทำการสั่งซื้อ"}
		for rows.Next() {
			var x CartItem
			if err = rows.Scan(&x.ID, &x.ProductID, &x.Name, &x.ImageURL, &x.Color, &x.Size, &x.Quantity, &x.PriceCents); err != nil {
				break
			}
			if ids[x.ID] {
				x.SubtotalCents = x.PriceCents * int64(x.Quantity)
				page.TotalCents += x.SubtotalCents
				page.CartItems = append(page.CartItems, x)
				page.CheckoutIDs = append(page.CheckoutIDs, x.ID)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		if len(page.CartItems) != len(ids) {
			fail(w, "สินค้าในตะกร้ามีการเปลี่ยนแปลง กรุณากลับไปเลือกใหม่")
			return
		}
		a.page(w, r, "checkout.html", page)
		return
	}
	paymentMethod := r.FormValue("payment_method")
	if paymentMethod != "promptpay" && paymentMethod != "mobile_banking" && paymentMethod != "cod" {
		fail(w, "กรุณาเลือกวิธีชำระเงิน")
		return
	}
	receiver := strings.TrimSpace(r.FormValue("receiver_name"))
	phone := strings.TrimSpace(r.FormValue("receiver_phone"))
	address := strings.TrimSpace(r.FormValue("shipping_address"))
	note := strings.TrimSpace(r.FormValue("order_note"))
	if len([]rune(receiver)) < 2 || len([]rune(receiver)) > 120 || len(phone) < 9 || len(phone) > 20 || len([]rune(address)) < 10 || len([]rune(address)) > 1000 || len([]rune(note)) > 500 {
		fail(w, "กรุณากรอกชื่อ เบอร์โทร และที่อยู่จัดส่งให้ครบถ้วน")
		return
	}
	for _, c := range phone {
		if (c < '0' || c > '9') && c != '+' && c != '-' && c != ' ' {
			fail(w, "เบอร์โทรไม่ถูกต้อง")
			return
		}
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT c.id,c.product_id,c.color,c.size,c.quantity,CAST(ROUND(p.price*100) AS SIGNED) FROM cart_items c JOIN products p ON p.id=c.product_id WHERE c.user_id=? ORDER BY c.id FOR UPDATE`, u.ID)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	type item struct {
		cartID, productID int64
		qty               int
		color, size       string
		price             int64
	}
	var items []item
	var total int64
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.cartID, &x.productID, &x.color, &x.size, &x.qty, &x.price); err != nil {
			break
		}
		if ids[x.cartID] {
			if x.qty < 1 || x.qty > 99 || x.price < 0 {
				err = errors.New("invalid cart")
				break
			}
			total += x.price * int64(x.qty)
			items = append(items, x)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "ไม่สามารถยืนยันคำสั่งซื้อ", 500)
		return
	}
	if len(items) != len(ids) {
		fail(w, "สินค้าในตะกร้ามีการเปลี่ยนแปลง กรุณากลับไปเลือกใหม่")
		return
	}
	res, err := tx.Exec(`INSERT INTO orders(user_id,total_cents,status,receiver_name,receiver_phone,shipping_address,order_note) VALUES (?,?,'pending',?,?,?,?)`, u.ID, total, receiver, phone, address, note)
	if err != nil {
		log.Print(err)
		http.Error(w, "database error (ตรวจว่า Import sql/checkout_shipping.sql แล้ว)", 500)
		return
	}
	orderID, err := res.LastInsertId()
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	if _, err = tx.Exec(`INSERT INTO order_payments(order_id,method,status) VALUES(?,?,'pending')`, orderID, paymentMethod); err != nil {
		log.Printf("payment insert: %v", err)
		http.Error(w, "database error (import sql/admin_messages_payments.sql)", 500)
		return
	}
	// Reserve inventory atomically inside the same transaction.
	// This also protects against two customers buying the last item simultaneously.
	for _, x := range items {
		res, stockErr := tx.Exec(`UPDATE products SET stock=stock-? WHERE id=? AND stock>=?`, x.qty, x.productID, x.qty)
		if stockErr != nil {
			http.Error(w, "database error", 500)
			return
		}
		n, stockErr := res.RowsAffected()
		if stockErr != nil || n != 1 {
			fail(w, "สินค้าในคลังไม่เพียงพอ กรุณาปรับจำนวนในตะกร้า")
			return
		}
	}
	for _, x := range items {
		if _, err = tx.Exec(`INSERT INTO order_items(order_id,product_id,color,size,quantity,unit_price_cents) VALUES (?,?,?,?,?,?)`, orderID, x.productID, x.color, x.size, x.qty, x.price); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		if _, err = tx.Exec(`DELETE FROM cart_items WHERE id=? AND user_id=?`, x.cartID, u.ID); err != nil {
			http.Error(w, "database error", 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	redirect(w, r, "/orders")
}

func (a *App) loadOrders(w http.ResponseWriter, r *http.Request, admin bool) {
	if !method(w, r, "GET") {
		return
	}
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}
	if !admin && u.Role == "admin" {
		http.Error(w, "เฉพาะลูกค้า", 403)
		return
	}
	if admin && u.Role != "admin" {
		http.Error(w, "เฉพาะผู้ดูแลระบบ", 403)
		return
	}
	p := Page{Title: "คำสั่งซื้อ", PromptPayQR: paymentQR(), PaymentAccount: paymentAccount()}
	query := `SELECT o.id,o.user_id,u.name,o.total_cents,o.status,DATE_FORMAT(o.created_at,'%d/%m/%Y %H:%i'),COALESCE(o.receiver_name,''),COALESCE(o.receiver_phone,''),COALESCE(o.shipping_address,''),COALESCE(o.order_note,''),COALESCE(p.method,''),COALESCE(p.status,'not_set'),(p.receipt_data IS NOT NULL AND OCTET_LENGTH(p.receipt_data)>0) FROM orders o JOIN users u ON u.id=o.user_id LEFT JOIN order_payments p ON p.order_id=o.id`
	var rows *sql.Rows
	var e error
	if admin {
		rows, e = a.db.Query(query + ` ORDER BY o.id DESC LIMIT 200`)
	} else {
		rows, e = a.db.Query(query+` WHERE o.user_id=? ORDER BY o.id DESC`, u.ID)
	}
	if e != nil {
		log.Printf("loadOrders query: %v", e)
		http.Error(w, "database error", 500)
		return
	}
	for rows.Next() {
		var o Order
		if e = rows.Scan(&o.ID, &o.UserID, &o.Customer, &o.TotalCents, &o.Status, &o.CreatedAt, &o.ReceiverName, &o.ReceiverPhone, &o.ShippingAddress, &o.OrderNote, &o.PaymentMethod, &o.PaymentStatus, &o.HasReceipt); e != nil {
			break
		}
		p.Orders = append(p.Orders, o)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		log.Printf("loadOrders scan: %v", e)
		http.Error(w, "database error", 500)
		return
	}
	for i := range p.Orders {
		o := &p.Orders[i]
		ir, er := a.db.Query(`SELECT COALESCE(oi.product_id,0),p.name,COALESCE(p.image_url,''),COALESCE(oi.color,''),COALESCE(oi.size,''),oi.quantity,oi.unit_price_cents FROM order_items oi LEFT JOIN products p ON p.id=oi.product_id WHERE oi.order_id=?`, o.ID)
		if er != nil {
			log.Printf("loadOrders items query (order %d): %v", o.ID, er)
			http.Error(w, "database error", 500)
			return
		}
		for ir.Next() {
			var x CartItem
			var name sql.NullString
			if er = ir.Scan(&x.ProductID, &name, &x.ImageURL, &x.Color, &x.Size, &x.Quantity, &x.PriceCents); er != nil {
				break
			}
			x.Name = name.String
			if !name.Valid {
				x.Name = "สินค้าที่ถูกลบ"
			}
			x.SubtotalCents = int64(x.Quantity) * x.PriceCents
			if !admin && x.ProductID > 0 && (o.Status == "confirmed" || o.Status == "completed") {
				var purchases, used int
				er = a.db.QueryRow(`SELECT COUNT(DISTINCT o.id) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE o.user_id=? AND oi.product_id=? AND o.status IN ('confirmed','completed')`, u.ID, x.ProductID).Scan(&purchases)
				if er != nil {
					break
				}
				er = a.db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE user_id=? AND product_id=?`, u.ID, x.ProductID).Scan(&used)
				if er != nil {
					break
				}
				x.CanReview = purchases > used
			}
			o.Items = append(o.Items, x)
		}
		if er == nil {
			er = ir.Err()
		}
		ir.Close()
		if er != nil {
			log.Printf("loadOrders items scan (order %d): %v", o.ID, er)
			http.Error(w, "database error", 500)
			return
		}
	}
	if admin {
		a.page(w, r, "admin_orders.html", p)
	} else {
		a.page(w, r, "orders.html", p)
	}
}
func (a *App) orders(w http.ResponseWriter, r *http.Request)      { a.loadOrders(w, r, false) }
func (a *App) adminOrders(w http.ResponseWriter, r *http.Request) { a.loadOrders(w, r, true) }
func (a *App) orderStatus(w http.ResponseWriter, r *http.Request) {
	u := a.postUser(w, r)
	if u == nil {
		return
	}
	if u.Role != "admin" {
		http.Error(w, "เฉพาะผู้ดูแลระบบ", 403)
		return
	}
	status := r.FormValue("status")
	if status != "pending" && status != "confirmed" && status != "completed" && status != "cancelled" {
		fail(w, "สถานะไม่ถูกต้อง")
		return
	}
	id := idFrom(r, "id")
	if id < 1 {
		fail(w, "เลขคำสั่งซื้อไม่ถูกต้อง")
		return
	}
	// Keep COD payment aligned when delivery is completed; never auto-verify bank transfers.
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`UPDATE orders SET status=? WHERE id=?`, status, id); e != nil {
		http.Error(w, "database error", 500)
		return
	}
	if status == "completed" {
		if _, e = tx.Exec(`UPDATE order_payments SET status='verified' WHERE order_id=? AND method='cod'`, id); e != nil {
			http.Error(w, "database error", 500)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "database error", 500)
		return
	}
	redirect(w, r, "/admin/orders")
}
