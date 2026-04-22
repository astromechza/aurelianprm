package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// entityRelTypes defines ordered display of contact entity sections.
var entityRelTypes = []struct {
	RelType    string
	EntityType string
	Label      string
}{
	{"hasEmail", "EmailAddress", "Email Addresses"},
	{"hasPhone", "PhoneNumber", "Phone Numbers"},
	{"hasWebSite", "WebSite", "Websites"},
	{"hasSocialNetwork", "SocialNetwork", "Social Networks"},
	{"hasAddress", "PostalAddress", "Postal Addresses"},
	{"hasCareer", "Career", "Career"},
	{"hasPet", "Pet", "Pets"},
}

// personRelTypes defines ordered display of person-to-person sections.
var personRelTypes = []struct {
	RelType string
	Label   string
}{
	{"knows", "Knows"},
	{"friendOf", "Friends"},
	{"spouseOf", "Spouse"},
	{"siblingOf", "Siblings"},
	{"parentOf", "Parent Of"},
}

// relTypeForEntityType maps an entity type to its relationship type.
func relTypeForEntityType(entityType string) string {
	for _, rt := range entityRelTypes {
		if rt.EntityType == entityType {
			return rt.RelType
		}
	}
	return ""
}

// labelForRelType returns the display label for a relationship type.
func labelForRelType(relType string) string {
	for _, rt := range entityRelTypes {
		if rt.RelType == relType {
			return rt.Label
		}
	}
	for _, rt := range personRelTypes {
		if rt.RelType == relType {
			return rt.Label
		}
	}
	return relType
}

// entityTypeForRelType maps a relationship type to its entity type.
func entityTypeForRelType(relType string) string {
	for _, rt := range entityRelTypes {
		if rt.RelType == relType {
			return rt.EntityType
		}
	}
	return ""
}

// nilOrStr returns nil for empty string, otherwise a pointer to the string.
func nilOrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseEntityFormData parses form values into validated JSON for the given entity type.
func parseEntityFormData(entityType string, r *http.Request) (json.RawMessage, error) {
	data := make(map[string]any)
	switch entityType {
	case "EmailAddress":
		email := r.FormValue("email")
		if email == "" {
			return nil, fmt.Errorf("email is required")
		}
		data["email"] = email
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "PhoneNumber":
		telephone := r.FormValue("telephone")
		if telephone == "" {
			return nil, fmt.Errorf("telephone is required")
		}
		data["telephone"] = telephone
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "WebSite":
		u := r.FormValue("url")
		if u == "" {
			return nil, fmt.Errorf("url is required")
		}
		data["url"] = u
		if title := r.FormValue("title"); title != "" {
			data["title"] = title
		}
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "SocialNetwork":
		platform := r.FormValue("platform")
		if platform == "" {
			return nil, fmt.Errorf("platform is required")
		}
		data["platform"] = platform
		if handle := r.FormValue("handle"); handle != "" {
			data["handle"] = handle
		}
		if u := r.FormValue("url"); u != "" {
			data["url"] = u
		}
	case "PostalAddress":
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
		if country := r.FormValue("country"); country != "" {
			data["country"] = country
		}
		if city := r.FormValue("city"); city != "" {
			data["city"] = city
		}
		if street := r.FormValue("streetAddress"); street != "" {
			data["streetAddress"] = street
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("at least one address field is required")
		}
	case "Career":
		role := r.FormValue("role")
		org := r.FormValue("organization")
		if role == "" || org == "" {
			return nil, fmt.Errorf("role and organization are required")
		}
		data["role"] = role
		data["organization"] = org
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "Pet":
		name := r.FormValue("name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		data["name"] = name
		if species := r.FormValue("species"); species != "" {
			data["species"] = species
		}
		if bd := r.FormValue("birthDate"); bd != "" {
			data["birthDate"] = bd
		}
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	default:
		return nil, fmt.Errorf("unknown entity type %q", entityType)
	}
	return json.Marshal(data)
}
