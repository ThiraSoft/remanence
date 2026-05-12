package frontend

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var content embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(content, "web")
	return http.FileServer(http.FS(sub))
}

func AboutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := content.ReadFile("web/about.html")
		if err != nil {
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
