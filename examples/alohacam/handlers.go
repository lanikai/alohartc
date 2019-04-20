package main

//go:generate go run util/generate_statics/main.go -o statics.go data/static
//go:generate go run util/generate_templates/main.go -o templates.go data/templates

import (
	"html/template"
	"mime"
	"net/http"
)

type staticHandler struct {
	http.Handler
}

func StaticServer() http.Handler {
	return &staticHandler{}
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set content type based on extension. Some browsers (e.g. Chrome) reject
	// assets with incorrect content type (e.g. CSS with text/plain).
	w.Header().Add("Content-Type", mime.TypeByExtension(r.URL.Path))
	w.Write(static(r.URL.Path))
}

// indexHandler serves index.tmpl
func indexHandler(w http.ResponseWriter, r *http.Request) {
	// `index` is generated via go:generate
	t := template.Must(template.New("index").Parse(index))
	t.Execute(w, nil)
}
