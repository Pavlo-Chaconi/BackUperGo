package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

//go:embed web/**
var webFS embed.FS

func main() {
	r := chi.NewRouter()

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(webFS, "web/assets")
	if err != nil {
		log.Fatal(err)
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "login.html")
	})
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "login.html")
	})
	r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "dashboard.html")
	})
	r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "settings.html")
	})
	r.Get("/schedule", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "schedule.html")
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))

	r.Route("/api", func(api chi.Router) {
		api.Get("/agent/poll", func(w http.ResponseWriter, r *http.Request) {
			agentID := r.URL.Query().Get("agent_id")
			resp := map[string]any{
				"action": "WAIT",
				"agent":  agentID,
				"ts":     time.Now().UTC().Format(time.RFC3339),
			}
			writeJSON(w, resp)
		})
		api.Post("/agent/report", func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			log.Printf("[REPORT] %+v", payload)
			writeJSON(w, map[string]any{"ok": true})
		})
		api.Post("/agent/event", func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			log.Printf("[EVENT] %+v", payload)
			writeJSON(w, map[string]any{"ok": true})
		})
	})

	log.Println("web-ui listening on http://127.0.0.1:8080")
	if err := http.ListenAndServe("127.0.0.1:8080", r); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
