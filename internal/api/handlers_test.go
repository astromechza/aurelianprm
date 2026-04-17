package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/api"
	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/db"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := &config.Config{
		Listen:           ":0",
		DBPath:           ":memory:",
		ReminderEmail:    "test@example.com",
		ReminderSendTime: "08:00",
		ReminderTimezone: "UTC",
	}

	handlers := api.NewHandlers(sqlDB, cfg)
	strict := api.NewStrictHandler(handlers, nil)
	mux := http.NewServeMux()
	api.HandlerFromMux(strict, mux)
	return mux
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

func do(t *testing.T, h http.Handler, method, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(context.Background(), method, path, body)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(context.Background(), method, path, nil)
	}
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListPeople_Empty(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/api/people", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var out []api.PersonSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Empty(t, out)
}

func TestCreateAndGetPerson(t *testing.T) {
	h := newTestHandler(t)

	body := jsonBody(t, map[string]any{"name": "Alice"})
	rec := do(t, h, http.MethodPost, "/api/people", body)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created api.PersonDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "Alice", created.Name)
	assert.Empty(t, created.ContactMethods)

	rec2 := do(t, h, http.MethodGet, "/api/people/"+created.Id, nil)
	require.Equal(t, http.StatusOK, rec2.Code)
	var got api.PersonDetail
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &got))
	assert.Equal(t, created.Id, got.Id)
}

func TestGetPerson_NotFound(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/api/people/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdatePerson(t *testing.T) {
	h := newTestHandler(t)
	body := jsonBody(t, map[string]any{"name": "Bob"})
	rec := do(t, h, http.MethodPost, "/api/people", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created api.PersonDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	update := jsonBody(t, map[string]any{"name": "Robert", "notes": "updated"})
	rec2 := do(t, h, http.MethodPut, "/api/people/"+created.Id, update)
	require.Equal(t, http.StatusOK, rec2.Code)
	var updated api.PersonDetail
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &updated))
	assert.Equal(t, "Robert", updated.Name)
}

func TestDeletePerson(t *testing.T) {
	h := newTestHandler(t)
	body := jsonBody(t, map[string]any{"name": "Carol"})
	rec := do(t, h, http.MethodPost, "/api/people", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created api.PersonDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec2 := do(t, h, http.MethodDelete, "/api/people/"+created.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec2.Code)

	rec3 := do(t, h, http.MethodGet, "/api/people/"+created.Id, nil)
	assert.Equal(t, http.StatusNotFound, rec3.Code)
}

func TestListPeople_Search(t *testing.T) {
	h := newTestHandler(t)
	do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Alice"}))
	do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Bob"}))

	rec := do(t, h, http.MethodGet, "/api/people?name=ali", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var out []api.PersonSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out, 1)
	assert.Equal(t, "Alice", out[0].Name)
}

func TestContactMethod_CreateAndDelete(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Dave"}))
	require.Equal(t, http.StatusCreated, rec.Code)
	var p api.PersonDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))

	cm := jsonBody(t, map[string]any{"type": "email", "value": "dave@example.com"})
	rec2 := do(t, h, http.MethodPost, "/api/people/"+p.Id+"/contact-methods", cm)
	require.Equal(t, http.StatusCreated, rec2.Code)
	var created api.ContactMethod
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &created))
	assert.Equal(t, "dave@example.com", created.Value)

	rec3 := do(t, h, http.MethodDelete, "/api/people/"+p.Id+"/contact-methods/"+created.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec3.Code)
}

func TestInteraction_CreateAndList(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Eve"}))
	require.Equal(t, http.StatusCreated, rec.Code)
	var p api.PersonDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))

	body := jsonBody(t, map[string]any{"occurred_at": "2025-06-01T10:00:00Z", "channel": "phone"})
	rec2 := do(t, h, http.MethodPost, "/api/people/"+p.Id+"/interactions", body)
	require.Equal(t, http.StatusCreated, rec2.Code)

	rec3 := do(t, h, http.MethodGet, "/api/people/"+p.Id+"/interactions", nil)
	require.Equal(t, http.StatusOK, rec3.Code)
	var out []api.Interaction
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &out))
	assert.Len(t, out, 1)
}

func TestPet_CRUD(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/api/pets", jsonBody(t, map[string]any{"name": "Rex"}))
	require.Equal(t, http.StatusCreated, rec.Code)
	var pet api.Pet
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pet))
	assert.Equal(t, "Rex", pet.Name)

	rec2 := do(t, h, http.MethodGet, "/api/pets/"+pet.Id, nil)
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := do(t, h, http.MethodPut, "/api/pets/"+pet.Id, jsonBody(t, map[string]any{"name": "Rexy"}))
	require.Equal(t, http.StatusOK, rec3.Code)

	rec4 := do(t, h, http.MethodDelete, "/api/pets/"+pet.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec4.Code)

	rec5 := do(t, h, http.MethodGet, "/api/pets/"+pet.Id, nil)
	assert.Equal(t, http.StatusNotFound, rec5.Code)
}

func TestWork_CRUD(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/api/work", jsonBody(t, map[string]any{"name": "ACME"}))
	require.Equal(t, http.StatusCreated, rec.Code)
	var work api.Work
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &work))

	rec2 := do(t, h, http.MethodPut, "/api/work/"+work.Id, jsonBody(t, map[string]any{"name": "ACME Corp", "website": "https://acme.com"}))
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := do(t, h, http.MethodDelete, "/api/work/"+work.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec3.Code)
}

func TestAddress_CRUD(t *testing.T) {
	h := newTestHandler(t)

	body := jsonBody(t, map[string]any{
		"label":    "home",
		"location": map[string]any{"country": "GB", "city": "London"},
	})
	rec := do(t, h, http.MethodPost, "/api/addresses", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	var addr api.Address
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &addr))
	assert.Equal(t, "GB", addr.Location.Country)

	rec2 := do(t, h, http.MethodGet, "/api/addresses/"+addr.Id, nil)
	require.Equal(t, http.StatusOK, rec2.Code)

	update := jsonBody(t, map[string]any{"location": map[string]any{"country": "DE", "city": "Berlin"}})
	rec3 := do(t, h, http.MethodPut, "/api/addresses/"+addr.Id, update)
	require.Equal(t, http.StatusOK, rec3.Code)
	var updated api.Address
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &updated))
	assert.Equal(t, "DE", updated.Location.Country)

	rec4 := do(t, h, http.MethodDelete, "/api/addresses/"+addr.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec4.Code)
}

func TestGetConfig(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/api/config", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var cfg api.ConfigResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cfg))
	assert.Equal(t, "UTC", cfg.ReminderTimezone)
}

func TestRelationship_CreateAndDelete(t *testing.T) {
	h := newTestHandler(t)
	r1 := do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Frank"}))
	var p1 api.PersonDetail
	require.NoError(t, json.Unmarshal(r1.Body.Bytes(), &p1))

	r2 := do(t, h, http.MethodPost, "/api/people", jsonBody(t, map[string]any{"name": "Grace"}))
	var p2 api.PersonDetail
	require.NoError(t, json.Unmarshal(r2.Body.Bytes(), &p2))

	relBody := jsonBody(t, map[string]any{
		"person_id": p1.Id,
		"to_type":   "person",
		"to_id":     p2.Id,
		"label":     "friend",
	})
	rec := do(t, h, http.MethodPost, "/api/relationships", relBody)
	require.Equal(t, http.StatusCreated, rec.Code)
	var rel api.Relationship
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rel))

	rec2 := do(t, h, http.MethodDelete, "/api/relationships/"+rel.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec2.Code)
}
