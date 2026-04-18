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

// BuildInfo holds version and runtime metadata displayed in the UI footer.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
	StartTime time.Time
}

// Server holds the HTTP server and its dependencies.
type Server struct {
	http      *http.Server
	handler   http.Handler
	templates *Templates
	db        *sql.DB
	buildInfo BuildInfo
}

// New constructs a Server, wires all routes, and wraps all routes (including
// static assets) in basic auth. rootFS must contain "templates/" and "static/"
// sub-trees (typically the embedded assets.FS).
//
// NOTE: deploy behind TLS in production — Basic Auth transmits credentials in
// base64 which is trivially decoded without transport encryption.
func New(cfg Config, database *sql.DB, rootFS fs.FS) (*Server, error) {
	templates := ParseTemplates(rootFS, cfg.BuildInfo)

	s := &Server{
		templates: templates,
		db:        database,
		buildInfo: cfg.BuildInfo,
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

	// People routes
	protected.HandleFunc("GET /people", s.handlePeopleList)
	protected.HandleFunc("GET /people/new", s.handlePersonNew)
	protected.HandleFunc("POST /people/new", s.handlePersonCreate)
	protected.HandleFunc("GET /people/{id}", s.handlePersonDetail)
	protected.HandleFunc("GET /people/{id}/edit", s.handlePersonEdit)
	protected.HandleFunc("POST /people/{id}/edit", s.handlePersonUpdate)
	protected.HandleFunc("POST /people/{id}/delete", s.handlePersonDelete)
	protected.HandleFunc("POST /people/{id}/interactions", s.handleInteractionCreate)
	protected.HandleFunc("POST /people/{id}/contact-methods", s.handleContactMethodCreate)
	protected.HandleFunc("DELETE /people/{id}/contact-methods/{method_id}", s.handleContactMethodDelete)
	protected.HandleFunc("GET /people/search", s.handlePersonSearch)
	protected.HandleFunc("POST /people/{id}/relationships", s.handleRelationshipCreate)
	protected.HandleFunc("GET /people/{id}/relationships/{rel_id}", s.handleRelationshipCard)
	protected.HandleFunc("GET /people/{id}/relationships/{rel_id}/edit", s.handleRelationshipEditForm)
	protected.HandleFunc("POST /people/{id}/relationships/{rel_id}/edit", s.handleRelationshipUpdate)
	protected.HandleFunc("DELETE /people/{id}/relationships/{rel_id}", s.handleRelationshipDelete)

	// Address routes
	protected.HandleFunc("GET /people/{id}/addresses/{addr_id}", s.handleAddressCard)
	protected.HandleFunc("GET /people/{id}/addresses/{addr_id}/edit", s.handleAddressEditForm)
	protected.HandleFunc("POST /people/{id}/addresses/{addr_id}/edit", s.handleAddressUpdate)
	protected.HandleFunc("POST /people/{id}/addresses", s.handleAddressCreate)
	protected.HandleFunc("DELETE /people/{id}/addresses/{addr_id}", s.handleAddressDelete)

	// Mount API handlers
	apiHandlers := api.NewHandlers(database, cfg.AppConfig)
	strictHandler := api.NewStrictHandler(apiHandlers, nil)
	api.HandlerFromMux(strictHandler, protected)

	mux := http.NewServeMux()
	if cfg.AppConfig != nil && cfg.AppConfig.DisableBasicAuth {
		log.Println("WARNING: Basic Auth is disabled — do not use this in production")
		mux.Handle("/", protected)
	} else {
		mux.Handle("/", BasicAuth(cfg.PasswordHash)(protected))
	}

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
	BuildInfo    BuildInfo
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
