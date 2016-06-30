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

	// Needs '/u' even /:id is a path separated param
	server.GET("/s/u/:id", s.GetRedirectHandler())
	server.GET("/s/info/:id", s.GetInfoHandler())
	server.POST("/s/register", s.GetSaveHandler())
	log.Print(http.ListenAndServe(":9999", server))
}
