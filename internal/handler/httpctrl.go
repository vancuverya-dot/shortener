package handler

import (
	"io"
	"net/http"
	"strings"

	"encoding/json"

	"github.com/sixafter/nanoid"
	"github.com/vancuverya-dot/shortener/internal/service"
)

var Urls map[string]string
var gen nanoid.Interface

type UrlRequest struct {
	Url string `json:"url"`
}

type UrlResponse struct {
	Result string `json:"result"`
}

func InitNanoId() {
	Urls = make(map[string]string)

	alphabet := "AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz0123456789"

	var err error
	gen, err = nanoid.NewGenerator(
		nanoid.WithAlphabet(alphabet),
		nanoid.WithLengthHint(10),
	)

	if err != nil {
		panic(err)
	}
}

func UrlPost(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, "bad_mime_type", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "500_error", http.StatusBadRequest)
		return
	}

	bodyString := string(bodyBytes)

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	Urls[id.String()] = bodyString

	w.Header().Set("Content-Type", "plain/text")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`http://localhost:8080/` + id.String()))
}

func UrlPostJson(w http.ResponseWriter, r *http.Request) {

	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") &&
		!strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/plain") {
		http.Error(w, "bad_mime_type", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	var urlRequest UrlRequest

	err := json.NewDecoder(r.Body).Decode(&urlRequest)

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Invalid JSON")
		http.Error(w, "shortener - Invalid JSON", http.StatusBadRequest)
		return
	}

	service.Log.Infof("Received URL: %s", urlRequest.Url)

	id, err := gen.New()

	if err != nil {
		service.Log.Errorw(err.Error(), "event", "shortener - Error generating Nano ID")
		return
	}

	Urls[id.String()] = urlRequest.Url

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	urlResonse := UrlResponse{
		Result: `http://localhost:8080/` + id.String(),
	}

	err = json.NewEncoder(w).Encode(urlResonse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func UrlGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	http.Redirect(w, r, Urls[id], http.StatusTemporaryRedirect)
}
