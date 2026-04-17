package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"

	"github.com/astromechza/aurelianprm/internal/store"
)

// Templates holds the parsed template set for the application.
type Templates struct {
	tmpl      *template.Template
	buildInfo BuildInfo
}

// ParseTemplates parses all templates from the given filesystem.
// It panics (via template.Must) if any template is malformed.
func ParseTemplates(fsys fs.FS, info BuildInfo) *Templates {
	// countryCodes returns a sorted slice of {Code, Name} for use in datalists.
	type countryOption struct{ Code, Name string }
	countryCodes := func() []countryOption {
		out := make([]countryOption, 0, len(store.Countries))
		for code, name := range store.Countries {
			out = append(out, countryOption{code, name})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}

	return &Templates{
		tmpl: template.Must(template.New("").Funcs(template.FuncMap{
			// deref dereferences a *string, returning "" for nil.
			"deref": func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			},
			// countryCodes returns all valid ISO 3166-1 alpha-2 codes sorted by name.
			"countryCodes": countryCodes,
		}).ParseFS(fsys, "templates/*.html")),
		buildInfo: info,
	}
}

// baseData is passed to base.html for full-page renders.
type baseData struct {
	Content   template.HTML // pre-rendered fragment, safe to embed
	Data      any           // original data, available to base layout if needed
	BuildInfo BuildInfo
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
		Content:   template.HTML(buf.String()), //nolint:gosec // content is produced by our own templates
		Data:      data,
		BuildInfo: t.buildInfo,
	}); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}
