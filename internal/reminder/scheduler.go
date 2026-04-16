package reminder

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Config holds the scheduler's dependencies and configuration.
type Config struct {
	// DB is the SQLite database connection.
	DB *sql.DB
	// SendFunc delivers the email. In production this wraps mailer.Send;
	// in tests it captures sent emails without SMTP.
	SendFunc func(subject, body string) error
	// SendTime is a wall-clock time in "HH:MM" format (e.g. "08:00").
	SendTime string
	// Timezone is a valid IANA timezone name (e.g. "Europe/London").
	Timezone string
}

// Scheduler runs a daily reminder digest goroutine.
type Scheduler struct {
	db         *sql.DB
	sendFunc   func(subject, body string) error
	sendHour   int
	sendMinute int
	tz         *time.Location
}

// NewScheduler creates a Scheduler from cfg. It panics if Timezone is invalid
// or SendTime is not in "HH:MM" format — both are configuration errors that
// should be caught at startup.
func NewScheduler(cfg Config) *Scheduler {
	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid timezone %q: %v", cfg.Timezone, err))
	}
	parts := strings.SplitN(cfg.SendTime, ":", 2)
	if len(parts) != 2 {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid send_time %q, expected HH:MM", cfg.SendTime))
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid send_time %q, expected HH:MM", cfg.SendTime))
	}
	return &Scheduler{
		db:         cfg.DB,
		sendFunc:   cfg.SendFunc,
		sendHour:   hour,
		sendMinute: minute,
		tz:         tz,
	}
}

// Run starts the scheduler loop. It sleeps until the next send time, runs the
// digest, then loops. It returns when ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	for {
		next := s.nextFireTime(time.Now())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(next)):
		}

		now := time.Now().In(s.tz)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.tz)
		s.runOnce(ctx, today)
	}
}

// nextFireTime returns the next wall-clock time to fire, based on now.
func (s *Scheduler) nextFireTime(now time.Time) time.Time {
	local := now.In(s.tz)
	candidate := time.Date(local.Year(), local.Month(), local.Day(),
		s.sendHour, s.sendMinute, 0, 0, s.tz)
	if now.Before(candidate) {
		return candidate
	}
	return candidate.AddDate(0, 0, 1)
}

// runOnce executes one daily digest cycle for the given today date.
func (s *Scheduler) runOnce(ctx context.Context, today time.Time) {
	// Determine the range of days to cover.
	startDay, err := s.computeStartDay(ctx, today)
	if err != nil {
		log.Printf("reminder: computing start day: %v", err)
		return
	}

	// Collect reminders for each day in range.
	var days []DayReminders
	for d := startDay; !d.After(today); d = d.AddDate(0, 0, 1) {
		items, err := ComputeReminders(ctx, s.db, d, s.tz)
		if err != nil {
			log.Printf("reminder: computing reminders for %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		if len(items) > 0 {
			days = append(days, DayReminders{Date: d, Items: items})
		}
	}

	// Nothing to send — stay silent, record nothing.
	if len(days) == 0 {
		return
	}

	subject := fmt.Sprintf("AurelianPRM \u2014 reminders for %s", today.Format("2 January 2006"))
	body := FormatDigest(today, days)

	// Attempt send with retries: immediate, +10s, +20s, +30s (4 total attempts).
	retryDelays := []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second}
	var sendErr error
	for i, delay := range retryDelays {
		if delay > 0 {
			time.Sleep(delay)
		}
		if sendErr = s.sendFunc(subject, body); sendErr == nil {
			break
		}
		log.Printf("reminder: send attempt %d/%d failed: %v", i+1, len(retryDelays), sendErr)
	}

	// Record outcome.
	status := "success"
	var errStr *string
	if sendErr != nil {
		status = "failure"
		msg := sendErr.Error()
		errStr = &msg
	}
	if err := store.CreateReminderHistory(ctx, s.db, model.ReminderHistory{
		Status:    status,
		Error:     errStr,
		DateStart: startDay,
		DateEnd:   today,
	}); err != nil {
		log.Printf("reminder: recording history: %v", err)
	}

	// Clean up old history only after a successful send.
	if sendErr == nil {
		cutoff := time.Now().AddDate(0, 0, -30)
		if err := store.DeleteOldReminderHistory(ctx, s.db, cutoff); err != nil {
			log.Printf("reminder: cleaning old history: %v", err)
		}
	}
}

// computeStartDay returns the first day to cover in this run.
// If no successful send has been recorded, it starts from today.
// Otherwise it starts from the day after the last successful send's date_end.
func (s *Scheduler) computeStartDay(ctx context.Context, today time.Time) (time.Time, error) {
	last, err := store.GetLastSuccessfulSend(ctx, s.db)
	if err != nil {
		return time.Time{}, err
	}
	if last == nil {
		return today, nil
	}
	// last.DateEnd is UTC midnight; shift to configured tz, add one day.
	lastEnd := last.DateEnd.In(s.tz)
	return time.Date(lastEnd.Year(), lastEnd.Month(), lastEnd.Day(), 0, 0, 0, 0, s.tz).AddDate(0, 0, 1), nil
}
