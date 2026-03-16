package tourrunner

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type Handler struct {
	Service     Service
	AllowedFrom string
	Logger      *log.Logger
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/lessons", h.handleLessons)
	mux.HandleFunc("/api/lessons/", h.handleLesson)
	mux.HandleFunc("/api/execute", h.handleExecute)
	return h.withCORS(mux)
}

func (h Handler) withCORS(next http.Handler) http.Handler {
	allowed := h.AllowedFrom
	if strings.TrimSpace(allowed) == "" {
		allowed = "*"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowed)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Handler) handleLessons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, h.Service.LessonSummaries())
}

func (h Handler) handleLesson(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/lessons/")
	lesson, ok := h.Service.Lessons[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "lesson not found"})
		return
	}
	writeJSON(w, http.StatusOK, lesson.Manifest)
}

func (h Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result, err := h.Service.Execute(r.Context(), req)
	if err != nil {
		status := http.StatusBadRequest
		if IsTimeout(err) {
			status = http.StatusGatewayTimeout
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
