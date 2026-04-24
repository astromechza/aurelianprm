package web

import (
	"encoding/json"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// PersonListItem is a decoded row for the persons list/search page.
type PersonListItem struct {
	ID         string
	Name       string
	NickName   string
	BirthYear  int
	BirthMonth int
}

// PersonListView is the view model for the persons list page.
type PersonListView struct {
	Query   string
	LinkRel string // non-empty when rendering for person-linking (returns link-persons-rows partial)
	Persons []PersonListItem
}

// PersonData holds decoded JSON fields of a Person entity.
type PersonData struct {
	Name       string `json:"name"`
	NickName   string `json:"nickName"`
	BirthYear  int    `json:"birthYear"`
	BirthMonth int    `json:"birthMonth"`
	BirthDay   int    `json:"birthDay"`
	Note       string `json:"note"`
}

// EntityWithRel pairs a contact entity with its connecting relationship and decoded data.
type EntityWithRel struct {
	Entity       dal.Entity
	Relationship dal.Relationship
	DataMap      map[string]any
}

// EntitySection is one grouped block of contact entities shown on the person detail page.
type EntitySection struct {
	PersonID   string
	RelType    string
	Label      string
	EntityType string // e.g. "EmailAddress"
	Entities   []EntityWithRel
}

// PersonWithRel pairs a person entity with its connecting relationship.
type PersonWithRel struct {
	Person       dal.Entity
	Relationship dal.Relationship
	Direction    string // "outbound" or "inbound"
}

// PersonRelSection groups person-to-person relationships by type.
type PersonRelSection struct {
	PersonID string
	RelType  string
	Label    string
	Persons  []PersonWithRel
}

// PersonDetailView is the view model for the person detail page.
type PersonDetailView struct {
	Person     dal.Entity
	PersonData PersonData
	Sections   []EntitySection
	People     []PersonRelSection
}

// EntityFormView is the view model for the add/edit contact entity partial.
type EntityFormView struct {
	PersonID string // for create mode (form action)
	EntityID string // for edit mode
	RelID    string // for edit mode
	Type     string // entity type e.g. "EmailAddress"
	DataMap  map[string]any
	DateFrom string // relationship date_from
	DateTo   string // relationship date_to
	EditMode bool
	Error    string
}

// RelationshipRowView is the view model for a single person-to-person relationship row partial.
type RelationshipRowView struct {
	PersonID      string
	PersonWithRel PersonWithRel
}

// RelationshipFormView is the view model for the add/edit person relationship partial.
type RelationshipFormView struct {
	PersonID    string
	RelID       string
	EditMode    bool
	DateFrom    string
	DateTo      string
	Note        string
	OtherPerson *dal.Entity
	Error       string
}

// decodeDataMap decodes entity JSON data into a plain map for template use.
func decodeDataMap(data json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if m == nil {
		m = make(map[string]any)
	}
	return m
}

// decodePersonData decodes a Person entity's JSON data into a typed struct.
func decodePersonData(data json.RawMessage) PersonData {
	var p PersonData
	_ = json.Unmarshal(data, &p)
	return p
}

// strOrEmpty dereferences a *string, returning "" for nil.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
