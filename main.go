package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	_ "github.com/lib/pq"
)

type Session struct {
	room string
	datetime time.Time
	movie Movie
}

type Movie struct {
	title string
	synopsis string
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
	_, err := db.Query("SELECT * FROM movies")

	if (err != nil) {
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Fprintf(w, "Hello!")
}

func main() {
	log.Println("Starting cinego server")
	defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/movies", listMovies).Methods(http.MethodGet)

	log.Println("Listening on port 80")
	http.ListenAndServe(":80", r)
}
