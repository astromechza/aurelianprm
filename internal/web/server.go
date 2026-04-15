package web

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/astromechza/aurelianprm/internal/api"
	"github.com/astromechza/aurelianprm/internal/config"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	http      *http.Server
	handler   http.Handler
	templates *Templates
	db        *sql.DB
}

// New constructs a Server, wires all routes, and wraps all routes (including
// static assets) in basic auth. rootFS must contain "templates/" and "static/"
// sub-trees (typically the embedded assets.FS).
//
// NOTE: deploy behind TLS in production — Basic Auth transmits credentials in
// base64 which is trivially decoded without transport encryption.
func New(cfg Config, database *sql.DB, rootFS fs.FS) (*Server, error) {
	templates := ParseTemplates(rootFS)

	s := &Server{
		templates: templates,
		db:        database,
	}

	staticSub, err := fs.Sub(rootFS, "static")
	if err != nil {
		return nil, fmt.Errorf("creating static sub-FS: %w", err)
	}

	// All routes are behind Basic Auth. Static assets are included — the
	// browser only requests them after it has already authenticated and
	// received the HTML page, so there is no bootstrapping problem.
	protected := http.NewServeMux()
	protected.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticSub)))
	protected.HandleFunc("GET /", s.handleIndex)

	// Mount API handlers
	apiHandlers := api.NewHandlers(database, cfg.AppConfig)
	strictHandler := api.NewStrictHandler(apiHandlers, nil)
	api.HandlerFromMux(strictHandler, protected)

	mux := http.NewServeMux()
	mux.Handle("/", BasicAuth(cfg.PasswordHash)(protected))

	s.handler = mux
	s.http = &http.Server{
		Addr:         cfg.Listen,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     log.Default(),
	}
	return s, nil
}

// ServeHTTP implements http.Handler so the server can be used directly in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Config holds web server configuration.
type Config struct {
	Listen       string
	PasswordHash string
	AppConfig    *config.Config
}

// ListenAndServe starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.http.Addr, err)
	}
	log.Printf("server listening on http://%s", ln.Addr())

	errCh := make(chan error, 1)
	go func() { errCh <- s.http.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// handleIndex renders the placeholder homepage.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.templates.Render(w, r, "index.html", nil)
}
