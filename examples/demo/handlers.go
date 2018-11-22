package main

//go:generate go run util/generate_statics.go -o statics.go data/static
//go:generate go run util/generate_templates.go -o templates.go data/templates

import (
	"html/template"
	"net/http"
)

type staticHandler struct {
	http.Handler
}

func StaticServer() http.Handler {
	return &staticHandler{}
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write(static(r.URL.Path))
}

// indexHandler serves index.tmpl
func indexHandler(w http.ResponseWriter, r *http.Request) {
	// `index` is generated via go:generate
	t := template.Must(template.New("index").Parse(index))
	t.Execute(w, nil)
}
