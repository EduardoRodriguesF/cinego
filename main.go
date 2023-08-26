package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

func listMovies(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db := ctx.Value("db").(*sql.DB)

		_, err := db.Query("SELECT * FROM movies")

		if (err != nil) {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "Hello!")
	}
}

func main() {
	log.Println("Starting cinego server")

	host := "db"
	username := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	db_name := os.Getenv("POSTGRES_DB")
	db_port := "5432"

	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, db_port, username, password, db_name)
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	ctx := context.WithValue(context.TODO(), "db", db)

	mux := http.NewServeMux()

	mux.HandleFunc("/movies", listMovies(ctx))

	log.Println("Listening on port 80")
	http.ListenAndServe(":80", mux)
}
