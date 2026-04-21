package schema

import "fmt"

// RelationshipTypeDef defines constraints for a relationship type.
type RelationshipTypeDef struct {
	Symmetric           bool
	AllowedSubjectTypes []string
	AllowedObjectTypes  []string
}

// RelationshipTypes is the authoritative registry of valid relationship types.
var RelationshipTypes = map[string]RelationshipTypeDef{
	"hasAddress":       {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"PostalAddress"}},
	"hasEmail":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"EmailAddress"}},
	"hasPhone":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"PhoneNumber"}},
	"hasWebSite":       {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"WebSite"}},
	"hasSocialNetwork": {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"SocialNetwork"}},
	"hasPet":           {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Pet"}},
	"hasCareer":        {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Career"}},
	"knows":            {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"friendOf":         {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"spouseOf":         {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"siblingOf":        {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"parentOf":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
}

// ValidateRelationship checks the relationship type exists and that the subject
// and object entity types satisfy its constraints.
func ValidateRelationship(relType, subjectEntityType, objectEntityType string) error {
	def, ok := RelationshipTypes[relType]
	if !ok {
		return fmt.Errorf("unknown relationship type %q", relType)
	}
	if err := checkAllowed(subjectEntityType, def.AllowedSubjectTypes); err != nil {
		return fmt.Errorf("relationship %q: subject type %q not allowed", relType, subjectEntityType)
	}
	if err := checkAllowed(objectEntityType, def.AllowedObjectTypes); err != nil {
		return fmt.Errorf("relationship %q: object type %q not allowed", relType, objectEntityType)
	}
	return nil
}

func checkAllowed(t string, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	for _, a := range allowed {
		if a == t {
			return nil
		}
	}
	return fmt.Errorf("%q not in %v", t, allowed)
}
