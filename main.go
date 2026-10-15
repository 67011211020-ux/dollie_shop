package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db     *sql.DB
	tmpl   *template.Template
	secure bool
}

type User struct {
	ID                int64
	Name, Email, Role string
}

type Category struct {
	ID                          int64
	Name, Emoji, Slug, ImageURL string
}

type Product struct {
	ID                          int64
	Name, Description, ImageURL string
	CategoryID                  int64
	Price                       float64
	Stock                       int
}

type Review struct {
	ID, ProductID                int64
	ProductName, Author, Comment string
	Rating                       int
	Owned                        bool
}

type Inquiry struct {
	ID                     int64
	Name, Subject, Message string
	Reply                  string
	RepliedAt              string
}

type Page struct {
	Title, Error, CSRF string
	User               *User
	Products           []Product
	Categories         []Category
	CategoryName       string
	SelectedCategory   string
	Product            Product
	Reviews            []Review
	Inquiries          []Inquiry
	CustomerMessages   []Inquiry
	Payments           []Payment
	PromptPayQR        string
	PaymentAccount     string
	EditReview         *Review
	Editing            bool
	CanReview          bool
	CartItems          []CartItem
	Orders             []Order
	TotalCents         int64
	CheckoutIDs        []int64
	Dashboard          DashboardStats
}

type CartItem struct {
	ID, ProductID             int64
	Name, ImageURL            string
	Color, Size               string
	Quantity                  int
	PriceCents, SubtotalCents int64
	CanReview                 bool
}

type Order struct {
	ID, UserID, TotalCents                                  int64
	Customer, Status, CreatedAt                             string
	ReceiverName, ReceiverPhone, ShippingAddress, OrderNote string
	PaymentMethod, PaymentStatus                            string
	HasReceipt                                              bool
	Items                                                   []CartItem
}

func main() {
	// อ่านค่าการเชื่อมต่อ MySQL
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("Please set MYSQL_DSN")
	}

	// อ่าน CA Certificate ของ Aiven
	caCert, err := os.ReadFile("ca.pem")
	if err != nil {
		log.Fatal("Cannot read Aiven CA certificate: ", err)
	}

	// สร้าง Certificate Pool
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		log.Fatal("Invalid Aiven CA certificate")
	}

	// ลงทะเบียน TLS Configuration ชื่อ aiven
	err = mysql.RegisterTLSConfig("aiven", &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		log.Fatal("Cannot register Aiven TLS: ", err)
	}

	// เชื่อมต่อฐานข้อมูล Aiven MySQL
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal("Cannot connect to Aiven MySQL: ", err)
	}

	log.Println("Connected to Aiven MySQL successfully")

	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"seqRatings": func() []int {
			return []int{1, 2, 3, 4, 5}
		},
		"stars": func(n int) string {
			if n < 0 {
				n = 0
			}
			if n > 5 {
				n = 5
			}
			return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
		},
		"money":  money,
		"colors": productColors,
		"sizes": func() []string {
			return []string{
				"1.0 เมตร",
				"1.2 เมตร",
				"1.4 เมตร",
				"1.6 เมตร",
				"1.8 เมตร",
				"2.0 เมตร",
			}
		},
	}).ParseGlob("templates/*.html")

	if err != nil {
		log.Fatal(err)
	}

	a := &App{
		db:     db,
		tmpl:   tmpl,
		secure: os.Getenv("COOKIE_SECURE") != "false",
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", a.home)
	mux.HandleFunc("/register", a.register)
	mux.HandleFunc("/login", a.login)
	mux.HandleFunc("/logout", a.logout)
	mux.HandleFunc("/product", a.product)
	mux.HandleFunc("/category", a.category)
	mux.HandleFunc("/review/save", a.saveReview)
	mux.HandleFunc("/review/delete", a.deleteReview)
	mux.HandleFunc("/contact", a.contact)
	mux.HandleFunc("/cart", a.cart)
	mux.HandleFunc("/cart/add", a.cartAdd)
	mux.HandleFunc("/cart/update", a.cartUpdate)
	mux.HandleFunc("/checkout", a.checkout)
	mux.HandleFunc("/orders", a.orders)
	mux.HandleFunc("/admin/orders", a.adminOrders)
	mux.HandleFunc("/admin/order/status", a.orderStatus)
	mux.HandleFunc("/admin", a.adminMenu)
	mux.HandleFunc("/admin/dashboard", a.admin)
	mux.HandleFunc("/admin/products", a.adminProducts)
	mux.HandleFunc("/admin/categories", a.adminCategories)
	mux.HandleFunc("/admin/messages", a.adminMessages)
	mux.HandleFunc("/admin/messages/reply", a.replyInquiry)
	mux.HandleFunc("/admin/payments", a.adminPayments)
	mux.HandleFunc("/admin/payments/status", a.paymentStatus)
	mux.HandleFunc("/payment/receipt", a.uploadReceipt)
	mux.HandleFunc("/payment/receipt/view", a.viewReceipt)
	mux.HandleFunc("/admin/category/save", a.saveCategory)
	mux.HandleFunc("/admin/category/delete", a.deleteCategory)
	mux.HandleFunc("/admin/product/save", a.saveProduct)
	mux.HandleFunc("/admin/product/delete", a.deleteProduct)

	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=3600")
			http.FileServer(http.Dir("static")).ServeHTTP(w, r)
		})),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           a.security(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Dollie Shop listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func (a *App) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; img-src 'self' https:; style-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
		)

		if a.secure {
			w.Header().Set(
				"Strict-Transport-Security",
				"max-age=31536000; includeSubDomains",
			)
		}

		if r.Method == http.MethodPost {
			limit := int64(1 << 20)

			if r.URL.Path == "/admin/category/save" ||
				r.URL.Path == "/payment/receipt" {
				limit = 5 << 20
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)

			var err error
			if r.URL.Path == "/admin/category/save" ||
				r.URL.Path == "/payment/receipt" {
				err = r.ParseMultipartForm(5 << 20)
			} else {
				err = r.ParseForm()
			}

			if err != nil {
				http.Error(w, "invalid form", 400)
				return
			}

			if !a.validCSRF(r) {
				http.Error(w, "CSRF validation failed", 403)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func digest(s string) string {
	v := sha256.Sum256([]byte(s))
	return hex.EncodeToString(v[:])
}

func (a *App) csrf(w http.ResponseWriter, r *http.Request) string {
	if c, e := r.Cookie("dollie_csrf"); e == nil && len(c.Value) == 64 {
		return c.Value
	}

	token, e := randomToken()
	if e != nil {
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "dollie_csrf",
		Value:    token,
		Path:     "/",
		Secure:   a.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return token
}

func (a *App) validCSRF(r *http.Request) bool {
	c, e := r.Cookie("dollie_csrf")
	if e != nil {
		return false
	}

	submitted := r.PostFormValue("csrf")

	return len(c.Value) == 64 &&
		len(submitted) == 64 &&
		subtle.ConstantTimeCompare(
			[]byte(c.Value),
			[]byte(submitted),
		) == 1
}

func (a *App) currentUser(r *http.Request) *User {
	c, e := r.Cookie("dollie_session")
	if e != nil || len(c.Value) != 64 {
		return nil
	}

	var u User

	err := a.db.QueryRow(
		`SELECT u.id,u.name,u.email,u.role
		 FROM sessions s
		 JOIN users u ON u.id=s.user_id
		 WHERE s.token_hash=? AND s.expires_at>NOW()`,
		digest(c.Value),
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role)

	if err != nil {
		return nil
	}

	return &u
}

func (a *App) page(w http.ResponseWriter, r *http.Request, name string, p Page) {
	p.User = a.currentUser(r)
	p.CSRF = a.csrf(w, r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if err := a.tmpl.ExecuteTemplate(w, name, p); err != nil {
		log.Printf("template: %v", err)
	}
}

func method(w http.ResponseWriter, r *http.Request, want string) bool {
	if r.Method != want {
		w.Header().Set("Allow", want)
		http.Error(w, "method not allowed", 405)
		return false
	}
	return true
}

func required(w http.ResponseWriter, u *User) bool {
	if u == nil {
		http.Error(w, "กรุณาเข้าสู่ระบบ", 401)
		return false
	}
	return true
}

func idFrom(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	if n < 1 {
		return 0
	}
	return n
}

func fail(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { log.Printf("home request completed in %s", time.Since(started)) }()
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if !method(w, r, "GET") {
		return
	}

	rows, e := a.db.Query(
		`SELECT id,name,description,price,image_url,
		        COALESCE(category_id,0),stock
		 FROM products
		 ORDER BY id DESC`,
	)
	if e != nil {
		log.Printf("home products query error: %v", e)
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()

	p := Page{
		Title:      "แค็ตตาล็อกตุ๊กตา",
		Categories: a.loadCategories(),
	}

	for rows.Next() {
		var x Product

		if scanErr := rows.Scan(
			&x.ID,
			&x.Name,
			&x.Description,
			&x.Price,
			&x.ImageURL,
			&x.CategoryID,
			&x.Stock,
		); scanErr != nil {
			log.Printf("home products scan error: %v", scanErr)
			http.Error(w, "database error", 500)
			return
		}

		p.Products = append(p.Products, x)
	}

	if rowErr := rows.Err(); rowErr != nil {
		log.Printf("home products rows error: %v", rowErr)
		http.Error(w, "database error", 500)
		return
	}

	a.page(w, r, "home.html", p)
}

func (a *App) category(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}

	slug := r.URL.Query().Get("type")
	name := ""
	var categoryID int64

	for _, c := range a.loadCategories() {
		if c.Slug == slug {
			name = c.Name
			categoryID = c.ID
			break
		}
	}

	if name == "" {
		http.NotFound(w, r)
		return
	}

	rows, err := a.db.Query(
		`SELECT id,name,description,price,image_url,
		        COALESCE(category_id,0),stock
		 FROM products
		 ORDER BY id DESC`,
	)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()

	page := Page{
		Title:            name,
		CategoryName:     name,
		SelectedCategory: slug,
		Categories:       a.loadCategories(),
	}

	for rows.Next() {
		var x Product

		if err = rows.Scan(
			&x.ID,
			&x.Name,
			&x.Description,
			&x.Price,
			&x.ImageURL,
			&x.CategoryID,
			&x.Stock,
		); err != nil {
			http.Error(w, "database error", 500)
			return
		}

		if x.CategoryID == categoryID {
			page.Products = append(page.Products, x)
		}
	}

	if rows.Err() != nil {
		http.Error(w, "database error", 500)
		return
	}

	a.page(w, r, "category.html", page)
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		a.page(w, r, "register.html", Page{Title: "สมัครสมาชิก"})
		return
	}

	if !method(w, r, "POST") {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	if len(name) < 2 ||
		len(name) > 100 ||
		len(email) > 190 ||
		!strings.Contains(email, "@") ||
		len(password) < 10 ||
		len(password) > 72 {
		fail(w, "ตรวจสอบชื่อ อีเมล และรหัสผ่าน (10–72 ตัวอักษร)")
		return
	}

	hash, e := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if e != nil {
		http.Error(w, "server error", 500)
		return
	}

	_, e = a.db.Exec(
		`INSERT INTO users(name,email,password_hash,role)
		 VALUES(?,?,?,'user')`,
		name,
		email,
		string(hash),
	)
	if e != nil {
		fail(w, "ไม่สามารถสมัครได้ อีเมลอาจถูกใช้แล้ว")
		return
	}

	redirect(w, r, "/login")
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		a.page(w, r, "login.html", Page{Title: "เข้าสู่ระบบ"})
		return
	}

	if !method(w, r, "POST") {
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))

	var id int64
	var hash string

	e := a.db.QueryRow(
		`SELECT id,password_hash FROM users WHERE email=?`,
		email,
	).Scan(&id, &hash)

	if e != nil ||
		bcrypt.CompareHashAndPassword(
			[]byte(hash),
			[]byte(r.FormValue("password")),
		) != nil {
		fail(w, "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}

	token, e := randomToken()
	if e != nil {
		http.Error(w, "server error", 500)
		return
	}

	_, e = a.db.Exec(
		`INSERT INTO sessions(token_hash,user_id,expires_at)
		 VALUES(?,?,?)`,
		digest(token),
		id,
		time.Now().Add(24*time.Hour),
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "dollie_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	redirect(w, r, "/")
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}

	if c, e := r.Cookie("dollie_session"); e == nil {
		_, _ = a.db.Exec(
			`DELETE FROM sessions WHERE token_hash=?`,
			digest(c.Value),
		)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "dollie_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	redirect(w, r, "/")
}

func (a *App) product(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}

	id := idFrom(r, "id")
	var p Page
	p.Title = "รายละเอียดตุ๊กตา"

	e := a.db.QueryRow(
		`SELECT id,name,description,price,image_url,stock
		 FROM products WHERE id=?`,
		id,
	).Scan(
		&p.Product.ID,
		&p.Product.Name,
		&p.Product.Description,
		&p.Product.Price,
		&p.Product.ImageURL,
		&p.Product.Stock,
	)

	if errors.Is(e, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}

	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	u := a.currentUser(r)

	if u != nil {
		var purchases, used int

		e = a.db.QueryRow(
			`SELECT COUNT(DISTINCT o.id)
			 FROM order_items oi
			 JOIN orders o ON o.id=oi.order_id
			 WHERE o.user_id=? AND oi.product_id=?
			 AND o.status IN ('confirmed','completed')`,
			u.ID,
			id,
		).Scan(&purchases)

		if e != nil {
			log.Printf("review purchases: %v", e)
			http.Error(w, "database error", 500)
			return
		}

		e = a.db.QueryRow(
			`SELECT COUNT(*)
			 FROM reviews
			 WHERE user_id=? AND product_id=?`,
			u.ID,
			id,
		).Scan(&used)

		if e != nil {
			log.Printf("review count: %v", e)
			http.Error(w, "database error", 500)
			return
		}

		p.CanReview = u.Role != "admin" && purchases > used
	}

	rows, e := a.db.Query(
		`SELECT r.id,r.product_id,u.name,r.rating,
		        r.comment,r.user_id
		 FROM reviews r
		 JOIN users u ON u.id=r.user_id
		 WHERE r.product_id=? AND r.deleted_at IS NULL
		 ORDER BY r.id DESC`,
		id,
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var v Review
		var owner int64

		if rows.Scan(
			&v.ID,
			&v.ProductID,
			&v.Author,
			&v.Rating,
			&v.Comment,
			&owner,
		) != nil {
			http.Error(w, "database error", 500)
			return
		}

		v.Owned = u != nil && u.Role != "admin" && u.ID == owner
		p.Reviews = append(p.Reviews, v)
	}

	if rows.Err() != nil {
		http.Error(w, "database error", 500)
		return
	}

	a.page(w, r, "product.html", p)
}

func (a *App) saveReview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}

	u := a.currentUser(r)
	if !required(w, u) {
		return
	}

	if u.Role == "admin" {
		http.Error(
			w,
			"admin cannot modify reviews",
			http.StatusForbidden,
		)
		return
	}

	pid := idFrom(r, "product_id")
	rid := idFrom(r, "id")
	rating, e := strconv.Atoi(r.FormValue("rating"))
	comment := strings.TrimSpace(r.FormValue("comment"))

	if pid == 0 ||
		e != nil ||
		rating < 1 ||
		rating > 5 ||
		len(comment) < 3 ||
		len(comment) > 2000 {
		fail(w, "ข้อมูลรีวิวไม่ถูกต้อง")
		return
	}

	if rid != 0 {
		result, err := a.db.ExecContext(
			r.Context(),
			`UPDATE reviews
			 SET rating=?,comment=?
			 WHERE id=? AND product_id=? AND user_id=?
			 AND deleted_at IS NULL`,
			rating,
			comment,
			rid,
			pid,
			u.ID,
		)

		if err != nil {
			http.Error(w, "database error", 500)
			return
		}

		n, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}

		if n == 0 {
			http.Error(
				w,
				"ไม่พบรีวิวของคุณหรือไม่มีสิทธิ์แก้ไข",
				http.StatusForbidden,
			)
			return
		}

		redirect(
			w,
			r,
			"/product?id="+strconv.FormatInt(pid, 10)+"#reviews",
		)
		return
	}

	// Lock customer row to serialize review submissions.
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer tx.Rollback()

	var lockedID int64

	if err = tx.QueryRow(
		`SELECT id FROM users WHERE id=? FOR UPDATE`,
		u.ID,
	).Scan(&lockedID); err != nil {
		log.Printf("review lock: %v", err)
		http.Error(w, "database error", 500)
		return
	}

	var purchases, used int

	if err = tx.QueryRow(
		`SELECT COUNT(DISTINCT o.id)
		 FROM order_items oi
		 JOIN orders o ON o.id=oi.order_id
		 WHERE o.user_id=? AND oi.product_id=?
		 AND o.status IN ('confirmed','completed')`,
		u.ID,
		pid,
	).Scan(&purchases); err != nil {
		log.Printf("review purchases: %v", err)
		http.Error(w, "database error", 500)
		return
	}

	if err = tx.QueryRow(
		`SELECT COUNT(*)
		 FROM reviews
		 WHERE user_id=? AND product_id=?`,
		u.ID,
		pid,
	).Scan(&used); err != nil {
		log.Printf("review count: %v", err)
		http.Error(w, "database error", 500)
		return
	}

	if purchases <= used {
		http.Error(
			w,
			"ใช้สิทธิ์รีวิวครบแล้ว ต้องสั่งซื้อสินค้านี้อีกครั้งจึงจะรีวิวใหม่ได้",
			http.StatusForbidden,
		)
		return
	}

	if _, err = tx.Exec(
		`INSERT INTO reviews(user_id,product_id,rating,comment)
		 VALUES(?,?,?,?)`,
		u.ID,
		pid,
		rating,
		comment,
	); err != nil {
		log.Printf("insert review: %v", err)
		http.Error(w, "database error", 500)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("commit review: %v", err)
		http.Error(w, "database error", 500)
		return
	}

	redirect(
		w,
		r,
		"/product?id="+strconv.FormatInt(pid, 10),
	)
}

func (a *App) deleteReview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}

	u := a.currentUser(r)
	if !required(w, u) {
		return
	}

	if u.Role == "admin" {
		http.Error(
			w,
			"admin cannot delete reviews",
			http.StatusForbidden,
		)
		return
	}

	rid := idFrom(r, "id")
	pid := idFrom(r, "product_id")

	if rid <= 0 || pid <= 0 {
		http.Error(w, "invalid review", 400)
		return
	}

	// Soft delete retains the review entitlement.
	result, err := a.db.ExecContext(
		r.Context(),
		`UPDATE reviews
		 SET deleted_at=NOW()
		 WHERE id=? AND product_id=? AND user_id=?
		 AND deleted_at IS NULL`,
		rid,
		pid,
		u.ID,
	)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}

	n, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}

	if n == 0 {
		http.Error(
			w,
			"ไม่พบรีวิวของคุณหรือไม่มีสิทธิ์ลบ",
			http.StatusForbidden,
		)
		return
	}

	redirect(
		w,
		r,
		"/product?id="+strconv.FormatInt(pid, 10)+"#reviews",
	)
}

func (a *App) contact(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if !required(w, u) {
		return
	}

	if r.Method == "GET" {
		msgs, err := a.loadCustomerMessages(w, r, u)
		if err != nil {
			log.Printf("contact messages: %v", err)
			http.Error(w, "database error", 500)
			return
		}

		a.page(
			w,
			r,
			"contact.html",
			Page{
				Title:            "ติดต่อร้าน",
				CustomerMessages: msgs,
			},
		)
		return
	}

	if !method(w, r, "POST") {
		return
	}

	subject := strings.TrimSpace(r.FormValue("subject"))
	message := strings.TrimSpace(r.FormValue("message"))

	if len(subject) < 3 ||
		len(subject) > 150 ||
		len(message) < 5 ||
		len(message) > 3000 {
		fail(w, "กรอกหัวข้อและข้อความให้ครบ")
		return
	}

	_, e := a.db.Exec(
		`INSERT INTO inquiries(user_id,subject,message)
		 VALUES(?,?,?)`,
		u.ID,
		subject,
		message,
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	redirect(w, r, "/contact?sent=1")
}

func (a *App) adminProducts(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}

	u := a.currentUser(r)
	if !required(w, u) {
		return
	}

	if u.Role != "admin" {
		http.Error(w, "เฉพาะผู้ดูแลระบบ", 403)
		return
	}

	p := Page{
		Title:      "แผงผู้ดูแล",
		Categories: a.loadCategories(),
	}

	rows, e := a.db.Query(
		`SELECT id,name,description,price,image_url,
		        COALESCE(category_id,0),stock
		 FROM products
		 ORDER BY id DESC`,
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	for rows.Next() {
		var v Product

		if rows.Scan(
			&v.ID,
			&v.Name,
			&v.Description,
			&v.Price,
			&v.ImageURL,
			&v.CategoryID,
			&v.Stock,
		) != nil {
			rows.Close()
			http.Error(w, "database error", 500)
			return
		}

		p.Products = append(p.Products, v)
	}

	e = rows.Err()
	rows.Close()

	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	ir, e := a.db.Query(
		`SELECT i.id,u.name,i.subject,i.message
		 FROM inquiries i
		 JOIN users u ON u.id=i.user_id
		 ORDER BY i.id DESC
		 LIMIT 100`,
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	for ir.Next() {
		var v Inquiry

		if ir.Scan(
			&v.ID,
			&v.Name,
			&v.Subject,
			&v.Message,
		) != nil {
			ir.Close()
			http.Error(w, "database error", 500)
			return
		}

		p.Inquiries = append(p.Inquiries, v)
	}

	e = ir.Err()
	ir.Close()

	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	a.page(w, r, "admin_products.html", p)
}

func (a *App) adminAllowed(w http.ResponseWriter, r *http.Request) bool {
	u := a.currentUser(r)
	if !required(w, u) {
		return false
	}

	if u.Role != "admin" {
		http.Error(w, "เฉพาะผู้ดูแลระบบ", 403)
		return false
	}

	return true
}

func (a *App) saveProduct(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}

	id := idFrom(r, "id")
	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	price, e := strconv.ParseFloat(r.FormValue("price"), 64)
	image := strings.TrimSpace(r.FormValue("image_url"))
	categoryID := idFrom(r, "category_id")
	stock, stockErr := strconv.Atoi(r.FormValue("stock"))

	if stockErr != nil || stock < 0 || stock > 1000000 {
		fail(w, "จำนวนสินค้าในคลังไม่ถูกต้อง")
		return
	}

	if categoryID == 0 {
		fail(w, "กรุณาเลือกหมวดหมู่สินค้า")
		return
	}

	var categoryExists int

	if a.db.QueryRow(
		`SELECT COUNT(*)
		 FROM categories
		 WHERE id=?`,
		categoryID,
	).Scan(&categoryExists) != nil || categoryExists == 0 {
		fail(w, "ไม่พบหมวดหมู่สินค้า")
		return
	}

	if len(name) < 2 ||
		len(name) > 120 ||
		len(desc) < 3 ||
		len(desc) > 5000 ||
		e != nil ||
		price < 0 ||
		price > 9999999 ||
		len(image) > 500 {
		fail(w, "ข้อมูลสินค้าไม่ถูกต้อง")
		return
	}

	if image != "" {
		parsed, e := url.Parse(image)

		if e != nil ||
			parsed.Scheme != "https" ||
			parsed.Host == "" ||
			parsed.User != nil {
			fail(w, "รูปภาพต้องเป็น HTTPS URL")
			return
		}
	}

	if id == 0 {
		_, e = a.db.Exec(
			`INSERT INTO products
			 (name,description,price,image_url,category_id,stock)
			 VALUES(?,?,?,?,?,?)`,
			name,
			desc,
			price,
			image,
			categoryID,
			stock,
		)
	} else {
		var result sql.Result

		result, e = a.db.Exec(
			`UPDATE products
			 SET name=?,description=?,price=?,image_url=?,
			     category_id=?,stock=?
			 WHERE id=?`,
			name,
			desc,
			price,
			image,
			categoryID,
			stock,
			id,
		)

		if e == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				http.NotFound(w, r)
				return
			}
		}
	}

	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	redirect(w, r, "/admin/products")
}

func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") || !a.adminAllowed(w, r) {
		return
	}

	_, e := a.db.Exec(
		`DELETE FROM products WHERE id=?`,
		idFrom(r, "id"),
	)
	if e != nil {
		http.Error(w, "database error", 500)
		return
	}

	redirect(w, r, "/admin/products")
}

// productColors returns the original five color options.
func productColors(productID int64) []string {
	values := []string{
		"น้ำตาลครีม",
		"ขาว",
		"ชมพู",
		"น้ำตาลอ่อน",
		"น้ำตาลเข้ม",
	}

	replacements := map[int64]string{
		6:  "ดำ",       // แพนด้า
		7:  "เหลือง",   // หมีพูห์
		24: "ครีม",     // ชิวาว่าสีครีม
		25: "เทา",      // ชเนาเซอร์สีเทา
		21: "สามสี",    // แมวหลายสี
		20: "เทา",      // แมวเหมียวสีเทา
		19: "คละสี",    // แมวขนฟูหลายสี
		18: "ขาวนวล",   // แมวอ้วนขนฟูสีอ่อน
		10: "ม่วงอ่อน", // StellaLou
	}

	if replacement, ok := replacements[productID]; ok {
		values[0] = replacement
	}

	return values
}
