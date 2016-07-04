package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/realfake/shrtie"
	backend "github.com/realfake/shrtie/backend/redis"
	redis "gopkg.in/redis.v4"
)

func main() {

	b, err := backend.New(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

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
