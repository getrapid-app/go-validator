package nethttp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/getrapid-app/go-validator/nethttp"
)

type CreatePost struct {
	Title string   `json:"title" validate:"required|string|max:100"`
	Tags  []string `json:"tags" validate:"array|max:3"`
}

func ExampleHandle() {
	mux := http.NewServeMux()
	mux.Handle("POST /posts", nethttp.Handle(func(w http.ResponseWriter, r *http.Request, req *CreatePost) {
		fmt.Fprintf(w, "created %q", req.Title)
	}))

	for _, body := range []string{`{"title":"Hello"}`, `{"tags":["a","b","c","d"]}`} {
		r := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		fmt.Println(w.Code, strings.TrimSpace(w.Body.String()))
	}
	// Output:
	// 200 created "Hello"
	// 422 {"message":"The title field is required. (and 1 more error)","errors":{"title":["The title field is required."],"tags":["The tags field must not have more than 3 items."]}}
}
