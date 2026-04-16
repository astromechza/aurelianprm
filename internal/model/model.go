// Package model defines the shared domain types for AurelianPRM.
package model

import "time"

// BlurryDate is a partially-specified date (year, month, and/or day may be
// absent). It also carries reminder configuration for anniversary-style events.
type BlurryDate struct {
	ID                 string
	Year               *int
	Month              *int // 1-12
	Day                *int // 1-31
	ReminderEnabled    bool
	ReminderDaysBefore int
}

// BlurryLocation is a geographic location with optional geocoordinates.
type BlurryLocation struct {
	ID        string
	Country   string // ISO 3166-1 alpha-2
	City      *string
	Street    *string
	Latitude  *float64
	Longitude *float64
}

// Person is the central entity of the PRM.
type Person struct {
	ID         string
	Name       string
	Nickname   *string
	Notes      *string
	BirthdayID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ContactMethod is a single email/phone/social/website entry for a person.
type ContactMethod struct {
	ID        string
	PersonID  string
	Type      ContactMethodType
	Value     string
	Label     *string
	CreatedAt time.Time
}

// ContactMethodType enumerates the allowed contact method kinds.
type ContactMethodType string

const (
	// ContactMethodEmail represents an email address.
	ContactMethodEmail ContactMethodType = "email"
	// ContactMethodPhone represents a phone number.
	ContactMethodPhone ContactMethodType = "phone"
	// ContactMethodSocial represents a social media handle or profile URL.
	ContactMethodSocial ContactMethodType = "social"
	// ContactMethodWebsite represents a personal or professional website URL.
	ContactMethodWebsite ContactMethodType = "website"
)

// Address is a labelled reference to a BlurryLocation.
type Address struct {
	ID         string
	LocationID string
	Label      *string
	Notes      *string
	CreatedAt  time.Time
}

// Pet is a named animal associated with one or more people via relationships.
type Pet struct {
	ID        string
	Name      string
	Notes     *string
	CreatedAt time.Time
}

// Work is a company or organisation associated with people via relationships.
type Work struct {
	ID        string
	Name      string
	Website   *string
	Notes     *string
	CreatedAt time.Time
}

// Interaction records a single contact event with a person.
type Interaction struct {
	ID         string
	PersonID   string
	OccurredAt time.Time
	Channel    *string
	Notes      *string
	CreatedAt  time.Time
}

// Relationship links a person to another entity (person, address, pet, work).
type Relationship struct {
	ID        string
	PersonID  string
	ToType    RelationshipTarget
	ToID      string
	Label     string
	SinceID   *string
	UntilID   *string
	Notes     *string
	CreatedAt time.Time
}

// RelationshipTarget enumerates the allowed target entity types.
type RelationshipTarget string

const (
	// RelationshipTargetPerson links a person to another person.
	RelationshipTargetPerson RelationshipTarget = "person"
	// RelationshipTargetAddress links a person to an address.
	RelationshipTargetAddress RelationshipTarget = "address"
	// RelationshipTargetPet links a person to a pet.
	RelationshipTargetPet RelationshipTarget = "pet"
	// RelationshipTargetWork links a person to a work/organisation.
	RelationshipTargetWork RelationshipTarget = "work"
)

// ReminderHistory records the outcome of a daily reminder digest attempt.
type ReminderHistory struct {
	ID        string
	SentAt    time.Time
	Status    string  // "success" | "failure"
	Error     *string // nil on success
	DateStart time.Time // calendar date; time component is always zero
	DateEnd   time.Time // calendar date; time component is always zero
}
