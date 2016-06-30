package shrtie

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Infoer interface {
	Info(string) (*Metadata, error)
}

type GetSaver interface {
	Get(string) (string, error)
	Save(string, time.Duration) string
}

type Metadata struct {
	Url     string    `json:"url"`
	TTL     int64     `json:"ttl,omitempty"`
	Clicked int64     `json:"click_count"`
	Created time.Time `json:"created"`
}

type entry struct {
	Url     string    `json:"url"`               // The URL to shorten
	TTL     int64     `json:"ttl,omitempty"`     // Time in seconds to life. Overwrites Expires
	Expires time.Time `json:"expires,omitempty"` // Sets the expiration date. Format is specified in RFC 3339
}

type ack struct {
	Url string `json:"url"` // The shortened URL
}

type shrtie struct {
	backend GetSaver
}

func New(backend GetSaver) shrtie {
	return shrtie{
		backend: backend,
	}
}

func (s shrtie) GetRedirectHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Get julienschmidt/httprouter path parameter
		// the is represents the (base64?) identifier used by the backend
		value, err := s.backend.Get(p.ByName("id"))
		if err != nil {
			http.Error(w, "Wrong Path", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, value, http.StatusMovedPermanently)
		return
	}
}

func (s shrtie) GetInfoHandler() httprouter.Handle {
	// Check if backend implements Infoer interface
	if backendInfo, ok := s.backend.(Infoer); ok {
		return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			// Get julienschmidt/httprouter path parameter
			// the is represents the (base64?) identifier used by the backend
			// Metadata is the returned struct of meta-infos to be sent back
			metadata, err := backendInfo.Info(p.ByName("id"))

			if err != nil {
				http.Error(w, "Wrong Path", http.StatusNotFound)
			}

			json.NewEncoder(w).Encode(metadata)
			return
		}
	}

	// Exit programm if backend doesn't support Infoer interface
	log.Fatalln("Backend doesn't support Infoer interface")
	return nil
}

func (s shrtie) GetSaveHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var request = entry{}
		var response = ack{}
		var ttl time.Duration

		// Check header (can be omitted)
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Wrong application", http.StatusBadRequest)
			return
		}

		// Read user Body JSON data
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Bad Data", http.StatusBadRequest)
			return
		}

		// Get TTL, if both (expiration date and ttl) are set ttl will overwrite date
		ttl = time.Duration(request.Expires.Unix()-time.Now().Unix()) * time.Second
		ttl = time.Duration(request.TTL) * time.Second

		if int64(ttl) < 0 {
			// Set ttl to inf
			ttl = time.Second * 0
		}

		key := s.backend.Save(request.Url, ttl)
		response.Url = concatURL(r, key)
		w.Header().Add("Content-Type", "application-json")
		json.NewEncoder(w).Encode(response)
		return
	}
}

func concatURL(r *http.Request, key string) string {
	if !r.URL.IsAbs() {
		// Not relative to response path
		// TODO: remove ?
		if r.TLS == nil {
			return fmt.Sprintf("http://%s/%s", r.Host, key)
		}
		return fmt.Sprintf("https://%s/%s", r.Host, key)
	}

	// No further checks function is only called by programm
	realativeUrl, _ := url.Parse(key)
	return r.URL.ResolveReference(realativeUrl).String()
}
