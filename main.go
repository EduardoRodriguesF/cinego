package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"

	_ "github.com/lib/pq"
)

type Session struct {
	Room string `json:"room"`
	Datetime time.Time `json:"datetime"`
	Movie Movie `json:"movie"`
}

type Movie struct {
	Slug string `json:"slug"`
	Title string `json:"title"`
	Synopsis string `json:"synopsis"`
}

var db *sql.DB

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")

	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	slug := reg.ReplaceAllString(s, "-")

	slug = strings.Trim(s, "-")

	return slug
}

func init() {
	host := "db"
	username := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	db_name := os.Getenv("POSTGRES_DB")
	db_port := "5432"

	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, db_port, username, password, db_name)
	log.Printf("Database Connection: %s", conn)

	var err error
	db, err = sql.Open("postgres", conn)

	if err != nil {
		log.Fatal(err)
	}
}

func listMovies(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT slug, title, synopsis FROM movies")
	var movies []Movie

	if (err != nil) {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	for rows.Next() {
		var movie Movie

		if err := rows.Scan(&movie.Slug, &movie.Title, &movie.Synopsis); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		movies = append(movies, movie)
	}

	res, err := json.Marshal(movies)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json")
	w.Write(res)

}

func createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var movie Movie
	if err := json.NewDecoder(r.Body).Decode(&movie); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	movie.Slug = slugify(movie.Title)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM movies WHERE slug = $1", movie.Slug).Scan(&count); err != nil {
		if err != sql.ErrNoRows {
			// Real error happened
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if count > 0 {
		http.Error(w, "Duplicate entry", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("INSERT INTO movies (slug, title, synopsis) VALUES ($1, $2, $3)", movie.Slug, movie.Title, movie.Synopsis)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func main() {
	log.Println("Starting cinego server")
	defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/movies", listMovies).Methods(http.MethodGet)
	r.HandleFunc("/movies", createMovieHandler).Methods(http.MethodPost)

	log.Println("Listening on port 80")
	http.ListenAndServe(":80", r)
}
