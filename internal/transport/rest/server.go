package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	core "github.com/mnemos-dev/mnemos/internal/core"
)

// Server is the REST HTTP server
type Server struct {
	mnemos *core.Mnemos
	logger *slog.Logger
	srv    *http.Server
}

func NewServer(mnemos *core.Mnemos, port int, logger *slog.Logger) *Server {
	s := &Server{mnemos: mnemos, logger: logger}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      corsMiddleware(loggingMiddleware(recoveryMiddleware(mux), logger)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	s.logger.Info("REST server starting", "addr", s.srv.Addr)
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	h := &handlers{mnemos: s.mnemos}
	mux.HandleFunc("POST /memories", h.storeMemory)
	mux.HandleFunc("GET /memories/{id}", h.getMemory)
	mux.HandleFunc("PATCH /memories/{id}", h.updateMemory)
	mux.HandleFunc("DELETE /memories/{id}", h.deleteMemory)
	mux.HandleFunc("GET /memories", h.listMemories)
	mux.HandleFunc("POST /memories/search", h.searchMemories)
	mux.HandleFunc("POST /memories/{id}/relate", h.relateMemory)
	mux.HandleFunc("GET /stats", h.getStats)
	mux.HandleFunc("POST /maintain", h.maintain)
}

// --- middleware ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func pathID(r *http.Request, segment string) string {
	// Go 1.22+ pattern matching: /memories/{id}
	parts := strings.Split(r.URL.Path, "/")
	for i, p := range parts {
		if p == segment && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return r.PathValue("id")
}
