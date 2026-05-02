// Package digest generates and sends email digests for upcoming reminders
// (birthdays and other scheduled events) derived from the stored entity data.
package digest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// personBirthData holds the birthday fields decoded from a Person entity's JSON data.
type personBirthData struct {
	Name       string `json:"name"`
	BirthYear  int    `json:"birthYear"`
	BirthMonth int    `json:"birthMonth"`
	BirthDay   int    `json:"birthDay"`
}

// findBirthdayReminders returns Reminders for persons whose birthday falls in
// [today, today+6] (inclusive). now is used as the reference point.
// Go normalises Feb 29 → Mar 1 for non-leap years automatically.
func findBirthdayReminders(persons []dal.Entity, now time.Time) []Reminder {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowEnd := today.AddDate(0, 0, 7) // exclusive upper bound

	var reminders []Reminder
	for _, entity := range persons {
		var pd personBirthData
		if err := json.Unmarshal(entity.Data, &pd); err != nil {
			continue
		}
		if pd.BirthMonth == 0 || pd.BirthDay == 0 {
			continue
		}
		if pd.Name == "" {
			continue
		}
		// Try this year then next year; take the first candidate that falls in window.
		for _, year := range []int{today.Year(), today.Year() + 1} {
			candidate := time.Date(year, time.Month(pd.BirthMonth), pd.BirthDay, 0, 0, 0, 0, today.Location())
			if !candidate.Before(today) && candidate.Before(windowEnd) {
				label := fmt.Sprintf("%s's birthday", pd.Name)
				if pd.BirthYear > 0 {
					label = fmt.Sprintf("%s's birthday (turning %d)", pd.Name, year-pd.BirthYear)
				}
				reminders = append(reminders, Reminder{Date: candidate, Label: label})
				break
			}
		}
	}
	return reminders
}
