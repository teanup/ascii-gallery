package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/teanup/ascii-gallery/handler"
	"github.com/teanup/ascii-gallery/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	s, err := store.New(dataDir)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}

	baseURL := os.Getenv("EXTERNAL_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}
	baseURL = strings.TrimRight(baseURL, "/")
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		log.Fatalf("EXTERNAL_URL must be a valid http(s) URL, got: %s", baseURL)
	}

	animHandler := handler.NewAnimationsHandler(s)
	webHandler := handler.NewWebHandler(s, baseURL)

	mux := http.NewServeMux()

	// Home page
	mux.HandleFunc("GET /", webHandler.ServeHome)

	// Animation CRUD
	mux.HandleFunc("GET /anim", animHandler.ListAnimations)
	mux.HandleFunc("GET /anim/{id}", animHandler.GetAnimation)
	mux.HandleFunc("POST /anim", animHandler.CreateAnimation)
	mux.HandleFunc("PUT /anim/{id}", animHandler.UpdateAnimation)
	mux.HandleFunc("DELETE /anim/{id}", animHandler.DeleteAnimation)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("ASCII Gallery server starting on :%s", port)
	log.Printf("Data directory: %s", dataDir)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
