package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/anthonyposchen/auosrs.git/templates"
)

//go:embed dist
var dist embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(fsys)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// for specifically the root / index
		// we want to replace the default with our own
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			templates.Index().Render(context.Background(), w)
			return
			// do a templ include!!
		}
		http.FileServer(getFileSystem()).ServeHTTP(w, r)
	})
	fmt.Println("Server is running on port 42069")
	fmt.Println(http.ListenAndServe(":42069", nil))
}
