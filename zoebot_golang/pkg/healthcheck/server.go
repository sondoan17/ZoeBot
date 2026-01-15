// Package healthcheck provides a minimal HTTP health check server.
package healthcheck

import (
	"context"
	"net/http"
	"time"
)

// Server is a minimal HTTP server for health checks.
type Server struct {
	server *http.Server
}

// New creates a new lightweight health check server.
func New(addr string) *Server {
	mux := http.NewServeMux()

	// Minimal response, no allocations
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return &Server{
		server: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadTimeout:       2 * time.Second,
			WriteTimeout:      2 * time.Second,
			IdleTimeout:       30 * time.Second,
			ReadHeaderTimeout: 1 * time.Second,
			MaxHeaderBytes:    1 << 10, // 1KB
		},
	}
}

// Start starts the health check server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
