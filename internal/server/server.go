package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"gotrash/internal/store"
	"gotrash/web"
)

type Server struct {
	store  *store.Store
	router *http.ServeMux
	addr   string
}

func NewServer(addr string, store *store.Store) (*Server, error) {
	router := http.NewServeMux()
	s := &Server{
		store:  store,
		router: router,
		addr:   addr,
	}

	// Register Handlers
	router.HandleFunc("GET /", s.handleIndex)
	router.HandleFunc("POST /api/upload", s.handleUpload)
	router.HandleFunc("GET /p/{id}", s.handleView)
	router.HandleFunc("DELETE /p/{id}", s.handleDelete)
	router.HandleFunc("GET /raw/{id}", s.handleRaw)

	// Serve static files from embedded FS
	staticFS, err := fs.Sub(web.Assets, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to load static sub-filesystem: %w", err)
	}
	// Go 1.22+ wildcard path suffix {path...} matches all sub-paths
	router.Handle("GET /static/{path...}", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	return s, nil
}

// Start boots the HTTP server with graceful timeouts.
func (s *Server) Start(ctx context.Context) error {
	// Wrap router with secure HTTP headers, logging, and recovery middleware
	handler := s.withSecurityHeaders(s.withLogging(s.withRecovery(s.router)))

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down HTTP server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server Shutdown error: %v", err)
		}
	}()

	log.Printf("Server starting on HTTP address: %s", s.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server start failed: %w", err)
	}

	return nil
}
