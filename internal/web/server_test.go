package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/assets"
	"github.com/astromechza/aurelianprm/internal/web"
)

// testPasswordHash is a bcrypt hash of "test" at cost 4, used only in tests.
// Do not use cost 4 in production (config requires cost 12 via hash-password).
const testPasswordHash = "$2a$04$3M6AoK9IrYcEh/nGk2ClJew.klPwB15qXov6vvKCsg5DYztGkx1ri"

func newTestServer(t *testing.T) *web.Server {
	t.Helper()
	srv, err := web.New(web.Config{
		Listen:       ":0",
		PasswordHash: testPasswordHash,
	}, nil, assets.FS)
	require.NoError(t, err)
	return srv
}

func TestTemplatesParse(t *testing.T) {
	// ParseTemplates panics on bad templates; constructing the server is enough.
	newTestServer(t)
}

func TestIndex_FullPage(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("", "test")
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "AurelianPRM")
}

func TestIndex_HTMXFragment(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("", "test")
	req.Header.Set("HX-Request", "true")
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Welcome to AurelianPRM")
	// Full layout should NOT be present for HTMX fragments.
	assert.NotContains(t, rec.Body.String(), "<html")
}

func TestIndex_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIndex_WrongPassword(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("", "wrong")
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStaticAssets_BehindAuth(t *testing.T) {
	srv := newTestServer(t)

	// Without credentials, static assets should also be blocked.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/static/css/custom.css", nil)
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// With credentials, static assets should be served.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/static/css/custom.css", nil)
	req.SetBasicAuth("", "test")
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotFound(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	req.SetBasicAuth("", "test")
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
