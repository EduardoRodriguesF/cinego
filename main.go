package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	Title string `json:"title"`
	Synopsis string `json:"synopsis"`
}

var db *sql.DB

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
	rows, err := db.Query("SELECT * FROM movies")
	var movies []Movie

	if (err != nil) {
		http.Error(w, err.Error(), 500)
		return
	}

	defer rows.Close()

	for rows.Next() {
		var movie Movie

		if err := rows.Scan(&movie.Title, &movie.Synopsis); err != nil {
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

	log.Printf("Inserting title: %s", movie.Title)

	_, err := db.Exec("INSERT INTO movies (title, synopsis) VALUES ($1, $2)", movie.Title, movie.Synopsis)
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
