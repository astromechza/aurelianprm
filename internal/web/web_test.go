package web_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *web.Server {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)
	return s
}

func doGet(t *testing.T, s *web.Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doGetHX(t *testing.T, s *web.Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doPost(t *testing.T, s *web.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doMethod(t *testing.T, s *web.Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

// createTestPerson inserts a Person entity directly via the DAL and returns its ID.
// DisplayName is set to name so the entity is indexed for FTS search.
func createTestPerson(t *testing.T, sqlDB *sql.DB, name string) string {
	t.Helper()
	d := dal.New(sqlDB)
	var id string
	err := d.WithTx(t.Context(), func(q *dal.Queries) error {
		e, err := q.CreateEntity(t.Context(), dal.CreateEntityParams{
			Type:        "Person",
			DisplayName: &name,
			Data:        []byte(`{"name":"` + name + `"}`),
		})
		if err != nil {
			return err
		}
		id = e.ID
		return nil
	})
	require.NoError(t, err)
	return id
}

func TestRootRedirects(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/persons", w.Header().Get("Location"))
}

func TestPersonsList_Empty(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "People")
	assert.Contains(t, w.Body.String(), "New Person")
}

func TestPersonsList_ShowsPersons(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	createTestPerson(t, sqlDB, "Alice Example")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGet(t, s, "/persons")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alice Example")
}

func TestPersonsSearch_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	createTestPerson(t, sqlDB, "Bob Builder")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGetHX(t, s, "/persons?q=Bob")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<!DOCTYPE")
	assert.Contains(t, w.Body.String(), "Bob Builder")
}

func TestPersonsCreate_SearchableAfterCreate(t *testing.T) {
	s := newTestServer(t)
	// Create a person via the handler
	w := doPost(t, s, "/persons", "name=Zara+Searchable")
	require.Equal(t, http.StatusSeeOther, w.Code)

	// Now search via HTMX partial
	w2 := doGetHX(t, s, "/persons?q=Zara")
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "Zara Searchable")
}
