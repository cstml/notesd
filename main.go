package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var storage_path = "data"
var port = "3333"

func init() {
	storage_path = os.Getenv("STORAGE_PATH")
	if storage_path == "" {
		storage_path = "data"
	}
	port = os.Getenv("PORT")
	if port == "" {
		port = "3333"
	}
}

func main() {

	if err := os.Mkdir(storage_path,0755); err != nil && err.Error() != fmt.Sprintf("mkdir %s: file exists", storage_path) {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		time := time.Now().Unix()
		filename := fmt.Sprintf("%s/%d", storage_path, time)
		file, err := os.Create(filename)
    if err != nil {
			http.Error(w, "Could not create file", http.StatusInternalServerError)
			return
    }
		defer file.Close()

		// Copy request body directly to file
    _, err = io.Copy(file, r.Body)
    if err != nil {
        http.Error(w, "Could not write to file", http.StatusInternalServerError)
        return
    }
    defer r.Body.Close()

		w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "%d", time)
	})

	r.Get("/{timestamp}", func(w http.ResponseWriter, r *http.Request) {
		timestamp := chi.URLParam(r, "timestamp")
		filename := fmt.Sprintf("%s/%s", storage_path, timestamp)

    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }
    defer file.Close()

    // Copy file contents to response
    _, err = io.Copy(w, file)
    if err != nil {
        http.Error(w, "Could not read file", http.StatusInternalServerError)
        return
    }
	})

	// List all files
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    files, err := os.ReadDir(storage_path)
    if err != nil {
			http.Error(w, "Could not read directory", http.StatusInternalServerError)
			return
    }

    w.Header().Set("Content-Type", "text/plain")
    for _, file := range files {
			if !file.IsDir() {
				fmt.Fprintln(w, file.Name())
			}
    }
	})

	r.Delete("/{timestamp}", func(w http.ResponseWriter, r *http.Request) {
    timestamp := chi.URLParam(r, "timestamp")
    filename := fmt.Sprintf("%s/%s", storage_path, timestamp)

    err := os.Remove(filename)
    if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("File deleted successfully"))
	})

	log.Printf("serving on :%s\n",port )
	http.ListenAndServe(":3333", r)
}
