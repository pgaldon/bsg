package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-redis/redis"
)

var templates = template.Must(template.ParseFiles("templates/edit.html", "templates/view.html", "templates/index.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

type Page struct {
	Title string
	Body  []byte
}

type IndexPage struct {
	Title  string
	Titles []string
}

type redisStore struct {
	client *redis.Client
}

func NewRedisStore() Store {
	client := redis.NewClient(&redis.Options{
		Addr:     "192.168.1.28:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	return &redisStore{
		client: client,
	}
}

func (r redisStore) Set(id string, session Session) error {
	bs, err := json.Marshal(session)
	if err != nil {
		return errors.Wrap(err, "failed to save session to redis")
	}

	if err := r.client.Set(id, bs, 0).Err(); err != nil {
		return errors.Wrap(err, "failed to save session to redis")
	}

	return nil
}

func (r redisStore) Get(id string) (Session, error) {
	var session Session

	bs, err := r.client.Get(id).Bytes()
	if err != nil {
		return session, errors.Wrap(err, "failed to get session from redis")
	}

	if err := json.Unmarshal(bs, &session); err != nil {
		return session, errors.Wrap(err, "failed to unmarshall session data")
	}

	return session, nil
}

func (p *Page) save() error {
	filename := "pages/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func welcome() (*IndexPage, error) {
	return &IndexPage{Title: "Welcome to BSG"}, nil
}

func listPages() (*IndexPage, error) {
	files, err := ioutil.ReadDir("pages/")
	if err != nil {
		return nil, err
	}
	var cleanFiles []string
	for _, f := range files {
		cleanFiles = append(cleanFiles, strings.Split(f.Name(), ".")[0])
	}

	return &IndexPage{Title: "Welcome Mofos", Titles: cleanFiles}, nil
}

func loadPage(title string) (*Page, error) {
	filename := "pages/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	page, err := welcome()

	err = templates.ExecuteTemplate(w, "index.html", page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
	sessionsStore = sessions.NewRedisStore()
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
