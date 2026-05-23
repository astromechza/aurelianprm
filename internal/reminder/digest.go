// Package reminder implements the daily digest scheduler and email logic.
package reminder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Item is a single reminder included in a digest email.
type Item struct {
	// Label is the human-readable description, e.g. "John Smith — birthday"
	// or "Jane & Bob — married, 5 years".
	Label string
	// OccursOn is the anniversary date (triggerDate + ReminderDaysBefore).
	OccursOn time.Time
	// PersonID is the ID of the primary person for this item, used to build
	// a deep link in HTML digest emails. Empty for items with no single person.
	PersonID string
}

// ComputeReminders returns all reminder items due on triggerDate in the given
// timezone. An item is due when triggerDate + blurryDate.ReminderDaysBefore
// falls on the blurryDate's (month, day).
//
// Orphaned blurry_dates (no matching person birthday or ongoing relationship)
// are silently skipped.
func ComputeReminders(ctx context.Context, sqlDB *sql.DB, triggerDate time.Time, tz *time.Location) ([]Item, error) {
	dates, err := store.ListReminderBlurryDates(ctx, sqlDB)
	if err != nil {
		return nil, fmt.Errorf("reminder.ComputeReminders: %w", err)
	}

	var items []Item
	for _, bd := range dates {
		// bd.Month and bd.Day are guaranteed non-nil by the store query.
		candidateDate := triggerDate.In(tz).AddDate(0, 0, bd.ReminderDaysBefore)
		if int(candidateDate.Month()) != *bd.Month || candidateDate.Day() != *bd.Day {
			continue
		}

		item, ok, err := resolveItem(ctx, sqlDB, bd, candidateDate)
		if err != nil {
			log.Printf("reminder: resolving blurry_date %s: %v", bd.ID, err)
			continue
		}
		if !ok {
			continue // orphaned — silently skip
		}
		items = append(items, item)
	}
	return items, nil
}

// resolveItem looks up the human context for a blurry_date (birthday or
// relationship anniversary) and builds a Item. Returns ok=false if the
// blurry_date is orphaned (no matching entity found).
func resolveItem(ctx context.Context, sqlDB *sql.DB, bd model.BlurryDate, occursOn time.Time) (Item, bool, error) {
	// Check birthday first.
	person, err := store.GetPersonByBirthdayID(ctx, sqlDB, bd.ID)
	if err == nil {
		return Item{
			Label:    person.Name + " \u2014 birthday",
			OccursOn: occursOn,
			PersonID: person.ID,
		}, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Item{}, false, err
	}

	// Check ongoing relationship anniversaries.
	rels, err := store.ListOngoingRelationshipsBySinceID(ctx, sqlDB, bd.ID)
	if err != nil {
		return Item{}, false, err
	}
	if len(rels) == 0 {
		return Item{}, false, nil // orphaned
	}

	rel := rels[0]
	label, err := buildRelationshipLabel(ctx, sqlDB, rel, occursOn, bd)
	if err != nil {
		return Item{}, false, err
	}
	return Item{Label: label, OccursOn: occursOn}, true, nil
}

// buildRelationshipLabel constructs a human-readable label for a relationship
// anniversary, including the years count if the blurry_date has a year set.
func buildRelationshipLabel(ctx context.Context, sqlDB *sql.DB, rel model.Relationship, occursOn time.Time, bd model.BlurryDate) (string, error) {
	person, err := store.GetPerson(ctx, sqlDB, rel.PersonID)
	if err != nil {
		return "", err
	}

	yearSuffix := ""
	if bd.Year != nil {
		years := occursOn.Year() - *bd.Year
		if years == 1 {
			yearSuffix = ", 1 year"
		} else if years > 1 {
			yearSuffix = fmt.Sprintf(", %d years", years)
		}
	}

	if rel.ToType == model.RelationshipTargetPerson {
		other, err := store.GetPerson(ctx, sqlDB, rel.ToID)
		if err == nil {
			return fmt.Sprintf("%s & %s \u2014 %s%s", person.Name, other.Name, rel.Label, yearSuffix), nil
		}
	}
	return fmt.Sprintf("%s \u2014 %s%s", person.Name, rel.Label, yearSuffix), nil
}
