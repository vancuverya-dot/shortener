package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sixafter/nanoid"
)

var urls map[string]string
var gen nanoid.Interface

// выполняется 1 раз до мэин
func init() {
	urls = make(map[string]string)

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

	// закроем стрим после чтения в конце метода
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "500_error", http.StatusBadRequest)
		return
	}

	bodyString := string(bodyBytes)

	id, err := gen.New() // or gen.NewWithLength(10)
	if err != nil {
		fmt.Println("Error generating Nano ID:", err)
		return
	}

	urls[id.String()] = bodyString
	fmt.Fprintf(w, "%s%s", `http://localhost:8080/`, id.String())
}

func UrlGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "%s", urls[id])

}
