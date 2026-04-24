package web_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
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

func doAPIJSON(t *testing.T, s *web.Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
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

func TestPersonsNew_ReturnsForm(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons/new")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `action="/persons"`)
}

func TestPersonsCreate_RedirectsToDetail(t *testing.T) {
	s := newTestServer(t)
	w := doPost(t, s, "/persons", "name=Alice+Test")
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/persons/")
}

func TestPersonsCreate_RequiresName(t *testing.T) {
	s := newTestServer(t)
	w := doPost(t, s, "/persons", "name=")
	assert.Equal(t, http.StatusOK, w.Code) // re-renders form
	assert.Contains(t, w.Body.String(), "required")
}

func TestPersonsDetail_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Carol Detail")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGet(t, s, "/persons/"+id)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Carol Detail")
}

func TestPersonsDetail_NotFound(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons/NOTEXIST")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPersonsUpdate_Redirects(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Dave Old")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doMethod(t, s, http.MethodPut, "/persons/"+id, "name=Dave+New")
	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestPersonsDelete_Redirects(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Eve Delete")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doMethod(t, s, http.MethodDelete, "/persons/"+id, "")
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/persons", w.Header().Get("Location"))
}

func TestEntitiesCreate_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Frank Entity")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	body := "type=EmailAddress&email=frank%40example.com&label=work"
	w := doPost(t, s, "/persons/"+personID+"/entities", body)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "frank@example.com")
	// Should NOT contain full layout
	assert.NotContains(t, w.Body.String(), "<!DOCTYPE")
}

func TestRelationshipsCreate_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personAID := createTestPerson(t, sqlDB, "Alice Rel")
	personBID := createTestPerson(t, sqlDB, "Bob Rel")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	body := fmt.Sprintf("person_b_id=%s&rel_type=knows", personBID)
	w := doPost(t, s, "/persons/"+personAID+"/relationships", body)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Bob Rel")
	assert.NotContains(t, w.Body.String(), "<!DOCTYPE")
}

func TestRelationshipsDelete_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personAID := createTestPerson(t, sqlDB, "Carol Rel")
	personBID := createTestPerson(t, sqlDB, "Dave Rel")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	// Create relationship via handler
	body := fmt.Sprintf("person_b_id=%s&rel_type=knows", personBID)
	w := doPost(t, s, "/persons/"+personAID+"/relationships", body)
	require.Equal(t, http.StatusOK, w.Code)

	// Extract relationship ID from /relationships/{rid}/ pattern in response
	respBody := w.Body.String()
	ridStart := strings.Index(respBody, "/relationships/")
	require.NotEqual(t, -1, ridStart, "response should contain /relationships/ link")
	after := respBody[ridStart+len("/relationships/"):]
	ridEnd := strings.IndexAny(after, "/?\"")
	require.NotEqual(t, -1, ridEnd, "should find end of relationship ID")
	rid := after[:ridEnd]

	w2 := doMethod(t, s, http.MethodDelete, "/relationships/"+rid, "")
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestEntitiesDelete_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Grace Delete")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	// Create entity via handler to get its ID
	body := "type=EmailAddress&email=grace%40example.com"
	w := doPost(t, s, "/persons/"+personID+"/entities", body)
	require.Equal(t, http.StatusOK, w.Code)

	// Extract entity ID from response (/entities/XXXX/edit pattern)
	respBody := w.Body.String()
	eidStart := strings.Index(respBody, "/entities/")
	require.NotEqual(t, -1, eidStart, "response should contain /entities/ link")
	eidEnd := strings.Index(respBody[eidStart+len("/entities/"):], "/")
	require.NotEqual(t, -1, eidEnd)
	eid := respBody[eidStart+len("/entities/") : eidStart+len("/entities/")+eidEnd]

	w2 := doMethod(t, s, http.MethodDelete, "/entities/"+eid, "")
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestAPISearchEntities(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	createTestPerson(t, sqlDB, "Alice API")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	// Create a Pet entity to test type filter exclusion
	petBody := map[string]any{
		"type":         "Pet",
		"display_name": "Alice API",
		"data":         map[string]any{"name": "Fido"},
	}
	w1 := doAPIJSON(t, s, http.MethodPost, "/api/entities", petBody)
	require.Equal(t, http.StatusCreated, w1.Code)

	w := doAPIJSON(t, s, http.MethodPost, "/api/search-entities", map[string]string{"q": "Alice", "type": "Person"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "Alice API", resp.Results[0]["display_name"])
}

func TestAPIGetEntity_Found(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Bob API")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doAPIJSON(t, s, http.MethodGet, "/api/entities/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var entity map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entity))
	assert.Equal(t, id, entity["id"])
	assert.Equal(t, "Person", entity["type"])
}

func TestAPIGetEntity_NotFound(t *testing.T) {
	s := newTestServer(t)
	w := doAPIJSON(t, s, http.MethodGet, "/api/entities/NOTEXIST", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPICreateEntity(t *testing.T) {
	s := newTestServer(t)
	body := map[string]any{
		"type":         "Person",
		"display_name": "Carol API",
		"data":         map[string]any{"name": "Carol API"},
	}
	w := doAPIJSON(t, s, http.MethodPost, "/api/entities", body)
	assert.Equal(t, http.StatusCreated, w.Code)
	var entity map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entity))
	assert.Equal(t, "Person", entity["type"])
	assert.Equal(t, "Carol API", entity["display_name"])
	assert.NotEmpty(t, entity["id"])
}

func TestAPIUpdateEntity(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Dave API")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	body := map[string]any{
		"display_name": "Dave API Updated",
		"data":         map[string]any{"name": "Dave API Updated"},
	}
	w := doAPIJSON(t, s, http.MethodPut, "/api/entities/"+id, body)
	assert.Equal(t, http.StatusOK, w.Code)
	var entity map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entity))
	assert.Equal(t, "Dave API Updated", entity["display_name"])
}

func TestAPIDeleteEntity(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Eve API")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doAPIJSON(t, s, http.MethodDelete, "/api/entities/"+id, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Confirm gone
	w2 := doAPIJSON(t, s, http.MethodGet, "/api/entities/"+id, nil)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestAPIDeleteEntity_NotFound(t *testing.T) {
	s := newTestServer(t)
	w := doAPIJSON(t, s, http.MethodDelete, "/api/entities/NOTEXIST", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
