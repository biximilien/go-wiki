package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
)

// valid path with title
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

type Page struct {
	ID    int64  `json:id`
	Title string `json:"title"`
	Body  []byte `json:"body"`
}

var templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html", "templates/navbar.html"))

func (p *Page) save(conn *pgx.Conn) error {
	query := "INSERT INTO pages (title, body) VALUES ($1, $2) ON CONFLICT ON CONSTRAINT title DO UPDATE SET body = $2"
	_, err := conn.Exec(context.Background(), query, p.Title, p.Body)
	if err != nil {
		return err
	}
	return nil
}

func loadPage(title string, conn *pgx.Conn) (*Page, error) {
	var id int64
	var body []byte
	query := "SELECT id, body FROM pages WHERE title=$1"
	err := conn.QueryRow(context.Background(), query, title).Scan(&id, &body)
	if err != nil {
		return nil, err
	}
	return &Page{ID: id, Title: title, Body: body}, nil
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string, *pgx.Conn), conn *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2], conn)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string, conn *pgx.Conn) {
	p, err := loadPage(title, conn)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string, conn *pgx.Conn) {
	p, err := loadPage(title, conn)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string, conn *pgx.Conn) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save(conn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func main() {
	fmt.Fprintf(os.Stdout, "Starting do wiki...\n")
	// Initiate DB connection
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	// Serve files in `public/css` directory
	fs := http.FileServer(http.Dir("./public/css"))
	http.Handle("/css/", http.StripPrefix("/css/", fs))

	// Wiki actions
	http.HandleFunc("/view/", makeHandler(viewHandler, conn))
	http.HandleFunc("/edit/", makeHandler(editHandler, conn))
	http.HandleFunc("/save/", makeHandler(saveHandler, conn))

	// redirect to home page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/view/FrontPage", http.StatusFound)
	})

	fmt.Fprintf(os.Stdout, "Up and running!\n")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
