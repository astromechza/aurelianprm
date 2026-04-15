package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

// Templates holds the parsed template set for the application.
type Templates struct {
	tmpl *template.Template
}

// ParseTemplates parses all templates from the given filesystem.
// It panics (via template.Must) if any template is malformed.
func ParseTemplates(fsys fs.FS) *Templates {
	return &Templates{
		tmpl: template.Must(template.New("").Funcs(template.FuncMap{
			// deref dereferences a *string, returning "" for nil.
			"deref": func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			},
		}).ParseFS(fsys, "templates/*.html")),
	}
}

// baseData is passed to base.html for full-page renders.
type baseData struct {
	Content template.HTML // pre-rendered fragment, safe to embed
	Data    any           // original data, available to base layout if needed
}

// Render writes a template response. If the request carries an HX-Request
// header the named fragment template is rendered directly; otherwise the
// fragment is first rendered to a buffer, then embedded into the full base
// layout via the Content field.
func (t *Templates) Render(w http.ResponseWriter, r *http.Request, fragment string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Header.Get("HX-Request") != "" {
		if err := t.tmpl.ExecuteTemplate(w, fragment, data); err != nil {
			http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Render the fragment into a buffer first so errors don't corrupt the
	// partial response already written to w.
	var buf bytes.Buffer
	if err := t.tmpl.ExecuteTemplate(&buf, fragment, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := t.tmpl.ExecuteTemplate(w, "base.html", baseData{
		Content: template.HTML(buf.String()), //nolint:gosec // content is produced by our own templates
		Data:    data,
	}); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}
