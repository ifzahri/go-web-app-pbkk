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

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

type Page struct {
	ID    int64
	Title string
	Body  []byte
}

func initDB() error {
	cfg := mysql.Config{
		User:   os.Getenv("root"),
		Passwd: os.Getenv("root"),
		Net:    "tcp",
		Addr:   "127.0.0.1:3306",
		DBName: "wiki",
	}

	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}

	pingErr := db.Ping()
	if pingErr != nil {
		return pingErr
	}
	fmt.Println("Connected to database!")
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
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
