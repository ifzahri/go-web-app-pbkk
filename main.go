package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var db *sql.DB

type Page struct {
	ID    int64
	Title string
	Body  []byte
}

func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using system environment")
	}
	return nil
}

func initDB() error {
	loadEnv()

	cfg := mysql.Config{
		User:   os.Getenv("DB_USER"),
		Passwd: os.Getenv("DB_PASS"),
		Net:    "tcp",
		Addr: fmt.Sprintf("%s:%s",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT")),
		DBName: os.Getenv("DB_NAME"),

		ParseTime:            true,
		AllowNativePasswords: true,
	}

	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingErr := db.Ping()
	if pingErr != nil {
		return fmt.Errorf("error connecting to the database: %v", pingErr)
	}

	log.Println("Successfully connected to database")
	return nil
}

func (p *Page) save() error {

	var err error
	if p.ID > 0 {
		_, err = db.Exec("UPDATE pages SET title = ?, body = ? WHERE id = ?", p.Title, p.Body, p.ID)
	} else {
		result, err := db.Exec("INSERT INTO pages (title, body) VALUES (?, ?)", p.Title, p.Body)
		if err != nil {
			return err
		}
		p.ID, err = result.LastInsertId()
	}
	return err
}

func loadPage(title string) (*Page, error) {
	var p Page
	err := db.QueryRow("SELECT id, title, body FROM pages WHERE title = ?", title).Scan(&p.ID, &p.Title, &p.Body)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles(
	filepath.Join("pages", "edit.html"),
	filepath.Join("pages", "view.html"),
))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {

	err := initDB()
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
