package shrtie

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

type testBackend struct{}

func (testBackend) Save(s string, t time.Duration) string { return "abc" }

func (testBackend) Get(s string) (string, error) {
	if s == "abc" {
		return "https://here.com", nil
	}
	return "", errors.New("error")
}

func (testBackend) Info(s string) (*Metadata, error) {
	if s == "abc" {
		return &meta, nil
	}
	return nil, errors.New("error")
}

type testBackendWithoutInfoer struct{}

func (testBackendWithoutInfoer) Save(s string, t time.Duration) (ss string) { return }

func (testBackendWithoutInfoer) Get(s string) (ss string, err error) { return }

var meta = Metadata{
	URL:     "https://here.com",
	TTL:     100,
	Clicked: 1,
	Created: time.Date(2000, time.January, 1, 1, 1, 1, 1, time.UTC),
}

var tb = testBackend{}

func TestRedirecter(t *testing.T) {

	// Setup
	shrt := New(tb)
	redirectHandler := shrt.RedirectHandler()

	// Not able to chain test here !?

	// Success test:
	// This test should succeed
	// HEADER: Content-Type: application/json
	// URL: http://example.com/abc
	//

	req, err := http.NewRequest("GET", "http://example.com/abc", nil)
	if err != nil {
		t.Error("Failed in redirect test:", err)
	}

	req.Header.Add("Content-Type", "application/json")
	res := httptest.NewRecorder()
	background := context.Background()
	ctx := context.WithValue(background, "id", "abc")
	redirectHandler.f(res, req, ctx)

	if res.Code != http.StatusMovedPermanently {
		t.Error("Wrong Status Value")
	}

	if loc := res.Header().Get("Location"); loc != "https://here.com" {
		t.Error("Wrong redirect location in success test:", loc)
	}

	// Fail test:
	// This test should fail because of an wrong request URL
	// HEADER: Content-Type: application/json
	// >> URL: http://example.com/aaa
	//

	req, err = http.NewRequest("GET", "http://example.com/aaa", nil)
	if err != nil {
		t.Error("Failed in redirect test:", err)
	}

	req.Header.Add("Content-Type", "application/json")
	res = httptest.NewRecorder()
	ctx = context.WithValue(background, "id", "aaa")
	redirectHandler.f(res, req, ctx)

	if loc := res.Header().Get("Location"); loc == "https://here.com" {
		t.Error("Wrong redirect location in failing test:", loc)
	}

	// Fail test:
	// This test should fail because of an wrong Header
	// >> HEADER: Content-Type: application/json
	// URL: http://example.com/aaa
	//

	req, err = http.NewRequest("GET", "http://example.com/abc", nil)
	if err != nil {
		t.Error("Failed in redirect test:", err)
	}

	req.Header.Add("Content-Type", "application/pdf")
	res = httptest.NewRecorder()
	ctx = context.WithValue(background, "id", "abc")
	redirectHandler.f(res, req, ctx)

	if loc := res.Header().Get("Location"); loc != "https://here.com" {
		t.Error("Wrong redirect location in failing test:", loc)
	}
}

func TestSave(t *testing.T) {
	// Setup
	shrt := New(tb)
	redirectHandler := shrt.SaveHandler()

	// Not able to chain test here !?

	// Success test:
	// This test should succeed
	// HEADER: Content-Type: application/json
	// REQUEST:
	// BODY: {
	//      "url": "https//here.com",
	// }
	// RESPONSE:
	// BODY: {
	//      "url": "http://example,com/abc",
	// }

	req, err := http.NewRequest("GET", "http://example.com/", strings.NewReader(`{"url":"http://here.com"}`))
	if err != nil {
		t.Error("Failed in save test:", err)
	}

	req.Header.Add("Content-Type", "application/json")
	res := httptest.NewRecorder()
	background := context.Background()
	redirectHandler.f(res, req, background)

	if res.Code != http.StatusOK {
		t.Error("Wrong Status Value", res.Code)
	}

	var resJSON = Ack{}
	if err = json.Unmarshal(res.Body.Bytes(), &resJSON); err != nil {
		t.Error(err)
	}
	if resJSON.URL != "http://example.com/abc" {
		t.Error("Wrong short link in save test: ", resJSON.URL)
	}
}

func TestSaveWrongContentType(t *testing.T) {
	// Setup
	shrt := New(tb)
	redirectHandler := shrt.SaveHandler()

	req, err := http.NewRequest("GET", "http://example.com/", strings.NewReader(`{"url":"http://here.com"}`))
	if err != nil {
		t.Error("Failed in save test:", err)
	}

	req.Header.Add("Content-Type", "application/pdf")
	res := httptest.NewRecorder()
	background := context.Background()
	redirectHandler.f(res, req, background)

	if res.Code == http.StatusOK {
		t.Error("Wrong Status Value", res.Code)
	}
}

func TestSaveWrongRequestBody(t *testing.T) {
	// Setup
	shrt := New(tb)
	redirectHandler := shrt.SaveHandler()

	req, err := http.NewRequest("GET", "http://example.com/", strings.NewReader(`{"aaa:"http://here.com"}`))
	if err != nil {
		t.Error("Failed in save test:", err)
	}

	req.Header.Add("Content-Type", "application/json")
	res := httptest.NewRecorder()
	background := context.Background()
	redirectHandler.f(res, req, background)

	if res.Code != http.StatusOK {
		//t.Error("Wrong Status Value", res.Code)
	}
}

func TestSaveBadTTL(t *testing.T) {
	// Setup
	shrt := New(tb)
	redirectHandler := shrt.SaveHandler()

	req, err := http.NewRequest("GET", "http://example.com/", strings.NewReader(`{"url":"http://here.com", "ttl":-111}`))
	if err != nil {
		t.Error("Failed in save test:", err)
	}

	req.Header.Add("Content-Type", "application/json")
	res := httptest.NewRecorder()
	background := context.Background()
	redirectHandler.f(res, req, background)

	if res.Code != http.StatusOK {
		//t.Error("Wrong Status Value", res.Code)
	}
}

func TestInfo(t *testing.T) {
	// Setup
	shrt := New(tb)
	infoHandler := shrt.InfoHandler()

	// Not able to chain test here !?

	// Success test:
	// This test should succeed
	// REQUEST: http://example.com/info/abc
	// RESPONSE:
	// HEADER: Content-Type: application/json
	// BODY: {
	//      "url": "http://example,com/abc",
	//      "ttl":100,
	//      "click_count":1,
	//      "created":"2000-01-01T01:01:01.000000001Z",
	// }

	req, err := http.NewRequest("GET", "http://example.com/info/abc", nil)
	if err != nil {
		t.Error("Failed in info test:", err)
	}

	background := context.Background()
	ctx := context.WithValue(background, "id", "abc")
	res := httptest.NewRecorder()
	infoHandler.f(res, req, ctx)

	if res.Code != http.StatusOK {
		t.Error("Wrong Status Value", res.Code)
	}

	var resJSON = Metadata{}
	if err = json.Unmarshal(res.Body.Bytes(), &resJSON); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(resJSON, meta) {
		t.Errorf("Failed in info response test: Got %##v\n expected %##v\n", resJSON, meta)
	}

	defer func() {
		if recover() == nil {
			t.Error("Couldn't recover panic form wrong backend interface")
		}
	}()

	shrtWithoutInfoer := New(testBackendWithoutInfoer{})
	infoHandler = shrtWithoutInfoer.InfoHandler()
}

func TestInfoBadPath(t *testing.T) {
	// Setup
	shrt := New(tb)
	infoHandler := shrt.InfoHandler()

	req, err := http.NewRequest("GET", "http://example.com/info/aaa", nil)
	if err != nil {
		t.Error("Failed in info test:", err)
	}

	background := context.Background()
	ctx := context.WithValue(background, "id", "aaa")
	res := httptest.NewRecorder()
	infoHandler.f(res, req, ctx)

	if res.Code == http.StatusOK {
		t.Error("Wrong Status Value", res.Code)
	}
}

func TestMuxHandler(t *testing.T) {
	shrt := New(tb)
	router := mux.NewRouter()
	router.Handle("/short/{id}", shrt.RedirectHandler().Mux()).Methods("GET")
	res := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/short/abc", nil)

	if err != nil {
		t.Error("Failed in gorilla mux handlertest: ", err)
	}

	router.ServeHTTP(res, req)
	if loc := res.Header().Get("Location"); loc != "https://here.com" {
		t.Error("Wrong redirect location in mux handler test:", loc)
	}
}

func TestHttprouterHandler(t *testing.T) {
	shrt := New(tb)
	router := httprouter.New()
	router.GET("/short/:id", shrt.RedirectHandler().Httprouter())
	res := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/short/abc", nil)

	if err != nil {
		t.Error("Failed in gorilla mux handlertest: ", err)
	}

	router.ServeHTTP(res, req)
	if loc := res.Header().Get("Location"); loc != "https://here.com" {
		t.Error("Wrong redirect location in mux handler test:", loc)
	}
}

func TestConcatURL(t *testing.T) {
	getRequest := func(s string, toggleTLS bool, host string) *http.Request {
		if req, err := http.NewRequest("Get", s, nil); err == nil {
			// Dirty but needed for the test
			// TODO: Improve
			if toggleTLS {
				if req.TLS != nil {
					req.TLS = nil
				} else {
					req.TLS = &tls.ConnectionState{}
				}
			}

			// Use when relative path is used
			if host != "" {
				req.Host = host
			}

			return req

		} else {
			t.Error(err)
		}
		return nil
	}

	tests := []struct {
		req *http.Request
		res string
	}{
		{
			req: getRequest("http://example.com", false, ""),
			res: "http://example.com/abc",
		}, {
			req: getRequest("https://example.com", false, ""),
			res: "https://example.com/abc",
		}, {
			req: getRequest("/", false, "example.com"),
			res: "http://example.com/abc",
		}, {
			req: getRequest("/", true, "example.com"),
			res: "https://example.com/abc",
		},
	}

	for _, test := range tests {
		if concatened := concatURL(test.req, "abc"); concatened != test.res {
			t.Errorf("Failed concatURL test: Expected '%s' got '%s'.\n", test.res, concatened)
		}
	}
}

func TestHttpMuxHandler(t *testing.T) {
	shrt := New(tb)
	router := http.NewServeMux()
	router.HandleFunc("/short/", shrt.RedirectHandler().ServerMux())
	res := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/short/abc", nil)

	if err != nil {
		t.Error("Failed in gorilla mux handlertest: ", err)
	}

	router.ServeHTTP(res, req)
	if loc := res.Header().Get("Location"); loc != "https://here.com" {
		t.Error("Wrong redirect location in mux handler test:", loc)
	}
}
