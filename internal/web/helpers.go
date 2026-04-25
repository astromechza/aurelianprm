package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var shortMonths = [13]string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// fmtDate renders a date string ("YYYY", "YYYY-MM", "YYYY-MM-DD") with short month names.
func fmtDate(s string) string {
	if s == "" {
		return ""
	}
	var y, m, d int
	if n, _ := fmt.Sscanf(s, "%d-%d-%d", &y, &m, &d); n == 3 && m >= 1 && m <= 12 {
		return fmt.Sprintf("%d %s %d", d, shortMonths[m], y)
	}
	if n, _ := fmt.Sscanf(s, "%d-%d", &y, &m); n == 2 && m >= 1 && m <= 12 {
		return fmt.Sprintf("%s %d", shortMonths[m], y)
	}
	return s
}

// fmtBirthDate renders year/month/day integers with short month names.
// Zero values are omitted: year=1990,month=3,day=15 → "15 Mar 1990".
func fmtBirthDate(year, month, day int) string {
	if year == 0 && month == 0 {
		return ""
	}
	var parts []string
	if month >= 1 && month <= 12 {
		if day > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", day, shortMonths[month]))
		} else {
			parts = append(parts, shortMonths[month])
		}
	}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%d", year))
	}
	return strings.Join(parts, " ")
}

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

// osmSearchURL builds an OpenStreetMap search URL from a PostalAddress data map.
func osmSearchURL(dm map[string]any) string {
	var parts []string
	for _, key := range []string{"streetAddress", "city", "country"} {
		if v, ok := dm[key].(string); ok && v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "https://www.openstreetmap.org/search?query=" + url.QueryEscape(strings.Join(parts, ", "))
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
