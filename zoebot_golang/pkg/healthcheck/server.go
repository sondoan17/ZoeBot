// Package healthcheck provides HTTP health check server for ZoeBot.
package healthcheck

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Server is an HTTP server for health checks.
type Server struct {
	server *http.Server
}

// New creates a new health check server.
func New(addr string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("I'm alive!"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"zoebot"}`))
	})

	return &Server{
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Start starts the health check server.
func (s *Server) Start() error {
	log.Printf("🌐 Health check server starting on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping health check server...")
	return s.server.Shutdown(ctx)
}
