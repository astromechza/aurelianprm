package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/astromechza/aurelianprm/internal/dal"
)

//go:embed templates
var templatesFS embed.FS

// Server holds application state and serves HTTP.
type Server struct {
	dal  *dal.DAL
	tmpl *template.Template
}

// NewServer constructs a Server, parsing all embedded templates.
func NewServer(d *dal.DAL) (*Server, error) {
	funcs := template.FuncMap{
		"personName": func(e dal.Entity) string {
			var data struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(e.Data, &data)
			return data.Name
		},
		"strOrEmpty":   strOrEmpty,
		"fmtDate":      fmtDate,
		"fmtBirthDate": fmtBirthDate,
		"osmSearchURL": osmSearchURL,
		"relDisplay": func(relType, direction string) string {
			if relType == "parentOf" && direction == "inbound" {
				return "Child Of"
			}
			return labelForRelType(relType)
		},
		"entityFormCreate": func(personID, entityType string) EntityFormView {
			return EntityFormView{
				PersonID: personID,
				Type:     entityType,
				DataMap:  make(map[string]any),
				EditMode: false,
			}
		},
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(
		templatesFS,
		"templates/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{dal: d, tmpl: tmpl}, nil
}

// Handler returns the chi router with all routes registered.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/persons", http.StatusFound)
	})

	r.Get("/backup", s.handleBackup)

	r.Get("/persons", s.handlePersonsList)
	r.Get("/persons/new", s.handlePersonsNew)
	r.Post("/persons", s.handlePersonsCreate)
	r.Get("/persons/{id}", s.handlePersonsDetail)
	r.Put("/persons/{id}", s.handlePersonsUpdate)
	r.Delete("/persons/{id}", s.handlePersonsDelete)
	r.Get("/persons/{id}/edit-form", s.handlePersonsEditForm)

	r.Post("/persons/{id}/entities", s.handleEntitiesCreate)
	r.Get("/entities/{eid}/edit", s.handleEntitiesEditForm)
	r.Get("/entities/{eid}/cancel", s.handleEntitiesCancel)
	r.Put("/entities/{eid}", s.handleEntitiesUpdate)
	r.Delete("/entities/{eid}", s.handleEntitiesDelete)

	r.Post("/persons/{id}/relationships", s.handleRelationshipsCreate)
	r.Post("/persons/{id}/relationships/new-person", s.handleRelationshipsCreateAndLink)
	r.Get("/relationships/{rid}/edit", s.handleRelationshipsEditForm)
	r.Get("/relationships/{rid}/cancel", s.handleRelationshipsCancel)
	r.Put("/relationships/{rid}", s.handleRelationshipsUpdate)
	r.Delete("/relationships/{rid}", s.handleRelationshipsDelete)

	r.Route("/api", func(r chi.Router) {
		r.Post("/search-entities", s.handleAPISearchEntities)
		r.Get("/entities/{id}", s.handleAPIGetEntity)
		r.Post("/entities", s.handleAPICreateEntity)
		r.Put("/entities/{id}", s.handleAPIUpdateEntity)
		r.Delete("/entities/{id}", s.handleAPIDeleteEntity)
		r.Get("/relationships/{rid}", s.handleAPIGetRelationship)
		r.Post("/relationships", s.handleAPICreateRelationship)
		r.Put("/relationships/{rid}", s.handleAPIUpdateRelationship)
		r.Delete("/relationships/{rid}", s.handleAPIDeleteRelationship)
	})

	return r
}

// render executes a named template, writing to w. Errors are logged.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %q: %v", name, err)
	}
}

// serverError logs err and responds with a plain 500.
func (s *Server) serverError(w http.ResponseWriter, _ *http.Request, err error) {
	log.Printf("handler error: %v", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// isHTMX returns true when the request was triggered by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
