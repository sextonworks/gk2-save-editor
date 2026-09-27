package main

import (
	"net/http"
	"strings"
)

const iconsPrefix = "/icons/"

func iconHandler(lookup func(name string) (string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := strings.CutPrefix(r.URL.Path, iconsPrefix)
		name, png := strings.CutSuffix(file, ".png")
		if !ok || !png || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		path, err := lookup(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "max-age=86400")
		http.ServeFile(w, r, path)
	})
}
