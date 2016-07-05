package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3"
	"github.com/realfake/shrtie"
	backend "github.com/realfake/shrtie/backend/sqlite3"
)

func main() {

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	b, err := backend.New(db)

	if err != nil {
		log.Fatal(err)
	}

	s := shrtie.New(b)
	server := httprouter.New()

	// Get RedirectHandler and warp it
	// into a julienschmidt/httprouter compatible handler function
	server.GET("/s/:id", s.RedirectHandler().Httprouter())
	server.POST("/s", s.SaveHandler().Httprouter())

	server.GET("/info/:id", s.InfoHandler().Httprouter())

	// Start server
	log.Print(http.ListenAndServe(":9999", server))

}
