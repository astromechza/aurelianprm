package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractRelID parses the relationship ID from a relationship_row.html fragment
// by looking for the hx-get edit URL pattern.
func extractRelID(t *testing.T, body, personID string) string {
	t.Helper()
	prefix := fmt.Sprintf(`hx-get="/people/%s/relationships/`, personID)
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "could not find relationship ID in response body:\n%s", body)
	start := idx + len(prefix)
	end := strings.Index(body[start:], "/")
	require.True(t, end >= 0, "could not find end of relationship ID")
	return body[start : start+end]
}

func TestRelationshipCreate_LinkExistingPerson(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "friend")
	assert.Contains(t, body, "Bob")
}

func TestRelationshipCreate_WithDates(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":       {"colleague"},
		"to_type":     {"person"},
		"to_id":       {bobID},
		"notes":       {"met at conference"},
		"since_year":  {"2022"},
		"since_month": {"3"},
		"since_day":   {"15"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "colleague")
	assert.Contains(t, body, "2022")
	assert.Contains(t, body, "met at conference")
}

func TestRelationshipCreate_CreateNewPerson(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":            {"partner"},
		"to_type":          {"person"},
		"create_new_person": {"on"},
		"new_person_name":  {"Charlie"},
		"new_person_notes": {"met in 2020"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "partner")
	assert.Contains(t, body, "Charlie")

	// Charlie should now appear in the people list.
	list := doWeb(t, srv, http.MethodGet, "/people", nil)
	assert.Contains(t, list.Body.String(), "Charlie")
}

func TestRelationshipCreate_MissingLabel(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestRelationshipCreate_MissingToID(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestRelationshipCreate_CreateNewPerson_MissingName(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":            {"friend"},
		"to_type":          {"person"},
		"create_new_person": {"on"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestRelationshipEditForm_Loads(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":       {"friend"},
		"to_type":     {"person"},
		"to_id":       {bobID},
		"since_year":  {"2021"},
		"since_month": {"6"},
		"since_day":   {"1"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+aliceID+"/relationships/"+relID+"/edit", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `value="friend"`)
	assert.Contains(t, body, `value="2021"`)
	assert.Contains(t, body, `value="6"`)
	assert.Contains(t, body, `value="1"`)
}

func TestRelationshipUpdate_Success(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	rec = doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships/"+relID+"/edit", url.Values{
		"label":       {"best friend"},
		"notes":       {"since uni"},
		"since_year":  {"2015"},
		"since_month": {"9"},
		"since_day":   {"1"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "best friend")
	assert.Contains(t, body, "since uni")
	assert.Contains(t, body, "2015")
}

func TestRelationshipUpdate_ClearsDates(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":       {"friend"},
		"to_type":     {"person"},
		"to_id":       {bobID},
		"since_year":  {"2019"},
		"since_month": {"1"},
		"since_day":   {"1"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	// Update without dates — clears since.
	rec = doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships/"+relID+"/edit", url.Values{
		"label": {"friend"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "2019")
}

func TestRelationshipUpdate_MissingLabel_ReturnsForm(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	rec = doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships/"+relID+"/edit", url.Values{
		"label": {""},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Label is required")
	assert.Contains(t, rec.Body.String(), "<form ")
}

func TestRelationshipCard_CancelRestoresRow(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+aliceID+"/relationships/"+relID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "friend")
	assert.Contains(t, rec.Body.String(), "Bob")
}

func TestRelationshipDelete_Success(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+aliceID+"/relationships/"+relID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Confirm it's gone from the detail page.
	detail := doWeb(t, srv, http.MethodGet, "/people/"+aliceID, nil)
	assert.NotContains(t, detail.Body.String(), relID)
}

func TestRelationshipDelete_WrongPerson_OK(t *testing.T) {
	srv := newWebTestServer(t)
	aliceID := createPersonForTest(t, srv, "Alice")
	carolID := createPersonForTest(t, srv, "Carol")
	bobID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+aliceID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {bobID},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	relID := extractRelID(t, rec.Body.String(), aliceID)

	// Carol trying to delete Alice's relationship — GetPersonRelationshipRow returns
	// not-found, handler returns 200 (idempotent).
	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+carolID+"/relationships/"+relID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Alice's relationship is still intact.
	detail := doWeb(t, srv, http.MethodGet, "/people/"+aliceID, nil)
	assert.Contains(t, detail.Body.String(), "friend")
}

func TestPersonSearch_ReturnsResults(t *testing.T) {
	srv := newWebTestServer(t)
	createPersonForTest(t, srv, "Alice Smith")
	createPersonForTest(t, srv, "Bob Jones")

	rec := doHTMX(t, srv, http.MethodGet, "/people/search?_search=alice", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Alice Smith")
	assert.NotContains(t, body, "Bob Jones")
}

func TestPersonSearch_RequiresHTMX(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people/search?_search=alice", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
