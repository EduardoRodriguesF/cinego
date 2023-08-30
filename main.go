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

	"github.com/lib/pq"
)

const DUPLICATE_KEY_ERR = "23505"

type Session struct {
	Id string `json:"id"`
	StartsAt time.Time `json:"starts_at"`
	MovieSlug    string     `json:"movie"`
}

type Movie struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
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

func queryMovieFromSlug(slug string) (Movie, error) {
	var movie Movie

	row := db.QueryRow("SELECT slug, title, synopsis FROM movies WHERE slug = $1", slug)
	if err := row.Scan(&movie.Slug, &movie.Title, &movie.Synopsis); err != nil {
		return movie, err
	}

	return movie, nil
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

	if err != nil {
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

func readMovieHandler(w http.ResponseWriter, r *http.Request) {
	var movie Movie

	params := mux.Vars(r)
	slug := params["slug"]

	movie, err := queryMovieFromSlug(slug)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "Not found", http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	res, err := json.Marshal(movie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json")
	w.Write(res)
}

func partialUpdateMovieHandler(w http.ResponseWriter, r *http.Request) {
	var fields map[string]string

	params := mux.Vars(r)
	slug := params["slug"]

	if _, err := queryMovieFromSlug(slug); err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "Not found", http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if value, ok := fields["title"]; ok {
		updatedSlug := slugify(value)
		fields["slug"] = updatedSlug
	}

	var queryList []string
	for k, v := range fields {
		queryList = append(queryList, fmt.Sprintf("%s = '%s'", k, v))
	}
	query := strings.Join(queryList, ", ")

	statement := fmt.Sprintf("UPDATE movies SET %s WHERE slug = '%s'", query, slug)
	log.Printf("Statement: %s", statement)

	if _, err := db.Query(statement); err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if string(pqErr.Code) == DUPLICATE_KEY_ERR {
				http.Error(w, "Duplicate entry", http.StatusBadRequest)
				return
			}
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	var movie Movie

	params := mux.Vars(r)
	slug := params["slug"]

	row := db.QueryRow("SELECT slug, title, synopsis FROM movies WHERE slug = $1", slug)
	if err := row.Scan(&movie.Slug, &movie.Title, &movie.Synopsis); err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "Not found", http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if _, err := db.Query("DELETE FROM movies WHERE slug = $1", slug); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func sessionsSearchHandler(w http.ResponseWriter, r *http.Request) {
	movieQuery := r.URL.Query().Get("movie")
	log.Printf("Search: %s", movieQuery)

	rows, err := db.Query("SELECT id, movie, starts_at FROM sessions WHERE movie LIKE $1", movieQuery)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session

		if err := rows.Scan(&session.Id, &session.MovieSlug, &session.StartsAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sessions = append(sessions, session)
	}

	res, err := json.Marshal(sessions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json")
	w.Write(res)
}

func sessionByIdHandler(w http.ResponseWriter, r *http.Request) {
	var session Session
	params := mux.Vars(r)

	id, ok := params["id"]

	if !ok {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	row := db.QueryRow("SELECT id, movie, starts_at FROM sessions WHERE id = $1", id)

	if err := row.Scan(&session.Id, &session.MovieSlug, &session.StartsAt); err != nil {
		switch err {
		case sql.ErrNoRows:
			http.Error(w, "Missing id", http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	res, err := json.Marshal(session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json")
	w.Write(res)
}

func main() {
	log.Println("Starting cinego server")
	defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/movies", listMovies).Methods(http.MethodGet)
	r.HandleFunc("/movies", createMovieHandler).Methods(http.MethodPost)
	r.HandleFunc("/movies/{slug}", readMovieHandler).Methods(http.MethodGet)
	r.HandleFunc("/movies/{slug}", partialUpdateMovieHandler).Methods(http.MethodPatch)
	r.HandleFunc("/movies/{slug}", deleteMovieHandler).Methods(http.MethodDelete)

	r.HandleFunc("/sessions/search", sessionsSearchHandler).Methods(http.MethodGet)
	r.HandleFunc("/sessions/{id}", sessionByIdHandler).Methods(http.MethodGet)

	log.Println("Listening on port 80")
	http.ListenAndServe(":80", r)
}
