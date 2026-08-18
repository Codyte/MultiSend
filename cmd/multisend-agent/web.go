package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var embeddedWebUI embed.FS

func webUIHandler() http.Handler {
	content, err := fs.Sub(embeddedWebUI, "web")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(content))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		files.ServeHTTP(w, r)
	})
}
