package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

//go:embed web/*
var content embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(content, "web")
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Static assets are immutable between deploys — let the browser
		// cache them so they are not refetched on every navigation.
		switch path.Ext(r.URL.Path) {
		case ".png", ".css", ".ico", ".webp", ".svg":
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		fileServer.ServeHTTP(w, r)
	})
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
