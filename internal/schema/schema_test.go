package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/astromechza/aurelianprm/internal/schema"
	"github.com/stretchr/testify/require"
)

func TestValidateEntity_valid(t *testing.T) {
	data := json.RawMessage(`{"name":"Alice Smith"}`)
	require.NoError(t, schema.ValidateEntity("Person", data))
}

func TestValidateEntity_missingRequired(t *testing.T) {
	data := json.RawMessage(`{}`) // missing required name
	err := schema.ValidateEntity("Person", data)
	require.Error(t, err)
}

func TestValidateEntity_additionalProperty(t *testing.T) {
	data := json.RawMessage(`{"name":"Alice","unknownField":"oops"}`)
	err := schema.ValidateEntity("Person", data)
	require.Error(t, err)
}

func TestValidateEntity_unknownType(t *testing.T) {
	err := schema.ValidateEntity("Spaceship", json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestValidateEntity_formatEmail(t *testing.T) {
	err := schema.ValidateEntity("EmailAddress", json.RawMessage(`{"email":"not-an-email"}`))
	require.Error(t, err)
}

func TestValidateEntity_formatURI(t *testing.T) {
	err := schema.ValidateEntity("WebSite", json.RawMessage(`{"url":"not a uri"}`))
	require.Error(t, err)
}

func TestValidateEntity_phonePattern(t *testing.T) {
	require.NoError(t, schema.ValidateEntity("PhoneNumber", json.RawMessage(`{"telephone":"+44 20 7946 0958"}`)))
	require.Error(t, schema.ValidateEntity("PhoneNumber", json.RawMessage(`{"telephone":"abc-not-a-phone"}`)))
}

func TestValidateEntity_minLength(t *testing.T) {
	err := schema.ValidateEntity("Person", json.RawMessage(`{"name":""}`))
	require.Error(t, err)
}

func TestKnownEntityTypes(t *testing.T) {
	types := schema.KnownEntityTypes()
	require.Contains(t, types, "Person")
	require.Contains(t, types, "Career")
	require.Len(t, types, 8)
}

func TestValidateRelationship_valid(t *testing.T) {
	require.NoError(t, schema.ValidateRelationship("hasEmail", "Person", "EmailAddress"))
}

func TestValidateRelationship_wrongObjectType(t *testing.T) {
	err := schema.ValidateRelationship("hasEmail", "Person", "Person")
	require.Error(t, err)
}

func TestValidateRelationship_unknownType(t *testing.T) {
	err := schema.ValidateRelationship("hasDragon", "Person", "Pet")
	require.Error(t, err)
}
