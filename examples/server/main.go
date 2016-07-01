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

	server.GET("/s/:id", s.GetRedirectHandler())
	server.POST("/s", s.GetSaveHandler())

	// Can't be "/s/info/:id"
	server.GET("/info/:id", s.GetInfoHandler())

	// Start server
	log.Print(http.ListenAndServe(":9999", server))
}
