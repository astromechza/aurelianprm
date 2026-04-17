package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/web"
)

// createPersonForTest creates a person via the form and returns its ID.
func createPersonForTest(t *testing.T, srv *web.Server, name string) string {
	t.Helper()
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {name}})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	loc := rec.Header().Get("Location")
	parts := strings.Split(loc, "/")
	return parts[len(parts)-1]
}

// extractAddrID parses the address ID out of an address_card.html fragment by
// looking for the hx-get edit URL pattern.
func extractAddrID(t *testing.T, body, personID string) string {
	t.Helper()
	prefix := fmt.Sprintf(`hx-get="/people/%s/addresses/`, personID)
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "could not find address ID in response body")
	start := idx + len(prefix)
	end := strings.Index(body[start:], "/")
	require.True(t, end >= 0, "could not find end of address ID")
	return body[start : start+end]
}

func TestAddressCreate_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Alice")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"GB"},
		"city":    {"London"},
		"label":   {"home"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "London, GB")
	assert.Contains(t, body, "home")

	// Person detail page shows the address.
	detail := doWeb(t, srv, http.MethodGet, "/people/"+personID, nil)
	assert.Contains(t, detail.Body.String(), "London, GB")
}

func TestAddressCreate_WithDates(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country":    {"DE"},
		"city":       {"Berlin"},
		"from_year":  {"2020"},
		"from_month": {"3"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Berlin, DE")
	assert.Contains(t, body, "2020")
}

func TestAddressCreate_MissingCountry(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Carol")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"city": {"London"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestAddressEditForm_Loads(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Dave")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"FR"},
		"city":    {"Paris"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+personID+"/addresses/"+addrID+"/edit", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `value="FR"`)
	assert.Contains(t, body, `value="Paris"`)
}

func TestAddressUpdate_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Eve")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"GB"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses/"+addrID+"/edit", url.Values{
		"country":    {"DE"},
		"city":       {"Berlin"},
		"label":      {"work"},
		"from_year":  {"2022"},
		"from_month": {"6"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Berlin, DE")
	assert.Contains(t, body, "work")
	assert.Contains(t, body, "2022")
}

func TestAddressUpdate_ClearsDate(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Frank")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country":    {"AU"},
		"from_year":  {"2019"},
		"from_month": {"1"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	// Update without from_year/from_month — clears the date.
	rec = doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses/"+addrID+"/edit", url.Values{
		"country": {"AU"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "2019")
}

func TestAddressDelete_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Grace")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"NZ"},
		"city":    {"Auckland"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+personID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	detail := doWeb(t, srv, http.MethodGet, "/people/"+personID, nil)
	assert.NotContains(t, detail.Body.String(), "Auckland")
}

func TestAddressCard_CancelRestoresCard(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Heidi")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"CA"},
		"city":    {"Toronto"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+personID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Toronto, CA")
}

func TestAddressUpdate_MissingCountry_ReturnsForm(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Ivan")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"GB"},
		"city":    {"Bristol"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	// Submit edit with empty country — should re-render the edit form with an error.
	rec = doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses/"+addrID+"/edit", url.Values{
		"country": {""},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Country is required")
	assert.Contains(t, body, `<form `) // edit form, not card
}

func TestAddressDelete_WrongPerson_NotFound(t *testing.T) {
	srv := newWebTestServer(t)
	person1ID := createPersonForTest(t, srv, "Ivan")
	person2ID := createPersonForTest(t, srv, "Judy")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+person1ID+"/addresses", url.Values{
		"country": {"JP"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), person1ID)

	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+person2ID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
