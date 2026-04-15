package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/assets"
	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
)

func newWebTestServer(t *testing.T) *web.Server {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	srv, err := web.New(web.Config{
		Listen:       ":0",
		PasswordHash: "",
		AppConfig: &config.Config{
			Listen:           ":0",
			DBPath:           ":memory:",
			DisableBasicAuth: true,
			ReminderEmail:    "test@example.com",
			ReminderSendTime: "08:00",
			ReminderTimezone: "UTC",
		},
	}, sqlDB, assets.FS)
	require.NoError(t, err)
	return srv
}

func doWeb(t *testing.T, srv *web.Server, method, path string, body url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, path, strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, path, nil)
		require.NoError(t, err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func doHTMX(t *testing.T, srv *web.Server, method, path string, body url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, path, strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, path, nil)
		require.NoError(t, err)
	}
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestPeopleList_Empty(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "No people yet")
}

func TestPeopleList_HTMX_LiveSearch(t *testing.T) {
	srv := newWebTestServer(t)

	// Create two people via the form.
	doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Alice"}})
	doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Bob"}})

	// HTMX search for "ali" should return only the rows fragment containing Alice.
	rec := doHTMX(t, srv, http.MethodGet, "/people?name=ali", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Alice")
	assert.NotContains(t, body, "Bob")
	// Full base layout should NOT be present in HTMX response.
	assert.NotContains(t, body, "<html")
}

func TestPersonNew_Form(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people/new", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "New Person")
}

func TestPersonCreate_And_Detail(t *testing.T) {
	srv := newWebTestServer(t)

	// POST creates the person and redirects.
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{
		"name":     {"Carol"},
		"nickname": {"CC"},
		"notes":    {"test notes"},
	})
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	location := rec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "/people/"), "redirect should be to /people/{id}")

	// Follow redirect to detail page.
	rec2 := doWeb(t, srv, http.MethodGet, location, nil)
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "Carol")
	assert.Contains(t, body, "CC")
	assert.Contains(t, body, "test notes")
}

func TestPersonCreate_EmptyName_Returns400(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {""}})
	// Returns the form with an error, not a redirect.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Name is required")
}

func TestPersonDetail_NotFound(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people/nonexistent-id", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPersonEdit_And_Update(t *testing.T) {
	srv := newWebTestServer(t)

	// Create.
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Dave"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	// GET edit form.
	rec2 := doWeb(t, srv, http.MethodGet, "/people/"+id+"/edit", nil)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Edit Dave")

	// POST update.
	rec3 := doWeb(t, srv, http.MethodPost, "/people/"+id+"/edit", url.Values{
		"name":  {"David"},
		"notes": {"updated"},
	})
	assert.Equal(t, http.StatusSeeOther, rec3.Code)

	// Verify update.
	rec4 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Contains(t, rec4.Body.String(), "David")
	assert.Contains(t, rec4.Body.String(), "updated")
}

func TestPersonDelete(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Eve"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodPost, "/people/"+id+"/delete", nil)
	assert.Equal(t, http.StatusSeeOther, rec2.Code)
	assert.Equal(t, "/people", rec2.Header().Get("Location"))

	rec3 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Equal(t, http.StatusNotFound, rec3.Code)
}

func TestInteractionCreate_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Frank"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doHTMX(t, srv, http.MethodPost, "/people/"+id+"/interactions", url.Values{
		"occurred_at": {"2025-06-01T10:00"},
		"channel":     {"phone"},
		"notes":       {"called about lunch"},
	})
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "2025-06-01")
	assert.Contains(t, body, "phone")
}

func TestContactMethod_CreateAndDelete_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Grace"}})
	personID := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	// Add contact method via HTMX.
	rec2 := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/contact-methods", url.Values{
		"type":  {"email"},
		"value": {"grace@example.com"},
	})
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "grace@example.com")

	// Extract contact method ID from the hx-delete attribute in the row.
	// The row contains: hx-delete="/people/{personID}/contact-methods/{id}"
	prefix := `/people/` + personID + `/contact-methods/`
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "hx-delete attribute not found in response")
	rest := body[idx+len(prefix):]
	methodID := rest[:strings.IndexAny(rest, `"' `)]

	// Delete via HTMX.
	rec3 := doHTMX(t, srv, http.MethodDelete, "/people/"+personID+"/contact-methods/"+methodID, nil)
	assert.Equal(t, http.StatusOK, rec3.Code)
}

func TestRelationship_CreateAndDelete_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec1 := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Heidi"}})
	p1ID := strings.TrimPrefix(rec1.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Ivan"}})
	p2ID := strings.TrimPrefix(rec2.Header().Get("Location"), "/people/")

	// Add relationship via HTMX.
	rec3 := doHTMX(t, srv, http.MethodPost, "/people/"+p1ID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {p2ID},
	})
	assert.Equal(t, http.StatusOK, rec3.Code)
	body := rec3.Body.String()
	assert.Contains(t, body, "friend")
	assert.Contains(t, body, "Ivan")

	// Extract relationship ID from hx-delete="/relationships/{id}".
	prefix := `/relationships/`
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "hx-delete attribute not found")
	rest := body[idx+len(prefix):]
	relID := rest[:strings.IndexAny(rest, `"' `)]

	// Delete via HTMX.
	rec4 := doHTMX(t, srv, http.MethodDelete, "/relationships/"+relID, nil)
	assert.Equal(t, http.StatusOK, rec4.Code)
}

func TestPersonCreate_WithBirthday(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{
		"name":                 {"Julia"},
		"birthday_year":        {"1990"},
		"birthday_month":       {"6"},
		"birthday_day":         {"15"},
		"reminder_enabled":     {"on"},
		"reminder_days_before": {"3"},
	})
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Contains(t, rec2.Body.String(), "15 June 1990")
}
