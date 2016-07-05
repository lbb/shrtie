package shrtie

import (
	"encoding/json"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/gorilla/mux"
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
	URL     string    `json:"url"`           // The shortened URL
	TTL     int64     `json:"ttl,omitempty"` // Time to life in seconds
	Clicked int64     `json:"click_count"`   // Click count
	Created time.Time `json:"created"`       // Created time the format is specified in RFC 3339
}

type Entry struct {
	URL     string    `json:"url"`               // The URL to shorten
	TTL     int64     `json:"ttl,omitempty"`     // Time in seconds to life. Overwrites Expires
	Expires time.Time `json:"expires,omitempty"` // Sets the expiration date. Format is specified in RFC 3339
}

type Ack struct {
	URL string `json:"url"` // The shortened URL
}

type Shrtie struct {
	backend GetSaver
}

type Handler struct {
	// Function handels request. Context contains the request id under the key "id" as string.
	f func(http.ResponseWriter, *http.Request, context.Context)
}

func (h Handler) Httprouter() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		background := context.Background()
		ctx := context.WithValue(background, "id", p.ByName("id"))
		h.f(w, r, ctx)
	}
}

func (h Handler) Mux() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		background := context.Background()
		ctx := context.WithValue(background, "id", mux.Vars(r)["id"])
		h.f(w, r, ctx)
	}
}

func (h Handler) ServerMux() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := path.Base(r.URL.Path)
		background := context.Background()
		ctx := context.WithValue(background, "id", id)
		h.f(w, r, ctx)
	}
}

func New(backend GetSaver) Shrtie {
	return Shrtie{
		backend: backend,
	}
}

func (s Shrtie) RedirectHandler() Handler {
	return Handler{
		f: func(w http.ResponseWriter, r *http.Request, ctx context.Context) {
			// Get julienschmidt/httprouter path parameter
			// the is represents the (base64?) identifier used by the backend
			value, err := s.backend.Get(ctx.Value("id").(string))
			if err != nil {
				http.Error(w, "Wrong Path", http.StatusNotFound)
				return
			}

			http.Redirect(w, r, value, http.StatusMovedPermanently)
			return
		},
	}
}

func (s Shrtie) InfoHandler() Handler {
	// Check if backend implements Infoer interface
	if backendInfo, ok := s.backend.(Infoer); ok {
		return Handler{
			f: func(w http.ResponseWriter, r *http.Request, ctx context.Context) {
				// Get julienschmidt/httprouter path parameter
				// the is represents the (base64?) identifier used by the backend
				// Metadata is the returned struct of meta-infos to be sent back
				metadata, err := backendInfo.Info(ctx.Value("id").(string))

				if err != nil {
					http.Error(w, "Wrong Path", http.StatusNotFound)
				}

				json.NewEncoder(w).Encode(metadata)
				return
			},
		}
	}

	// Exit programm if backend doesn't support Infoer interface
	log.Panicln("Backend doesn't support Infoer interface")
	return Handler{}
}

func (s Shrtie) SaveHandler() Handler {
	return Handler{
		f: func(w http.ResponseWriter, r *http.Request, _ context.Context) {
			var request = Entry{}
			var response = Ack{}
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

			key := s.backend.Save(request.URL, ttl)
			response.URL = concatURL(r, key)
			w.Header().Add("Content-Type", "application-json")
			json.NewEncoder(w).Encode(response)
			return
		},
	}
}

func concatURL(r *http.Request, key string) string {
	absURL := r.URL
	if !r.URL.IsAbs() {
		if r.TLS == nil {
			absURL, _ = url.Parse("http://" + r.Host + r.URL.String())
		} else {
			absURL, _ = url.Parse("https://" + r.Host + r.URL.String())
		}
	}

	// Add tailing slash to extend path properly
	// host/path/ => host/path/abc instead of
	// host/path => host/abc
	// If user has set url fragment or url query this will fail!
	// User is not supposed to do this
	// absUrl.Fragment = ""
	// absUrl.RawQuery = ""
	absURL.Path = absURL.Path + "/"

	// No further checks function is only called by programm
	realativeURL, _ := url.Parse(key)
	return absURL.ResolveReference(realativeURL).String()
}
