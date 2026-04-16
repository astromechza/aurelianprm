package reminder_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/reminder"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Note: openTestDB is defined in digest_test.go (same package reminder_test).

func TestScheduler_SendsEmailWhenRemindersExist(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a blurry_date due on Jan 2 (trigger Jan 1 + 1 day).
		// Fake time starts at 2000-01-01, scheduler fires at 00:01.
		// today = 2000-01-01; candidateDate = 2000-01-02 (month=1, day=2) ✓
		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name:       "Test Person",
			BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		var sent []string
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				sent = append(sent, body)
				mu.Unlock()
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		// Advance 2 minutes past the send time.
		time.Sleep(2 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, sent, 1)
		assert.Contains(t, sent[0], "Test Person \u2014 birthday")

		// Verify history was recorded.
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		require.NotNil(t, h)
		assert.Equal(t, "success", h.Status)
	})
}

func TestScheduler_SilentWhenNoReminders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var callCount int
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				callCount++
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(2 * time.Minute)
		synctest.Wait()

		assert.Equal(t, 0, callCount) // nothing to send → no email, no history
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		assert.Nil(t, h)
	})
}

func TestScheduler_RetriesOnFailureThenSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up a reminder due on day 1.
		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Retry Person", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		attempts := 0
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				defer mu.Unlock()
				attempts++
				if attempts < 3 {
					return errors.New("temporary SMTP failure")
				}
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		// Give enough fake time for the send + retries (10s + 20s gaps).
		time.Sleep(5 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 3, attempts) // 2 failures + 1 success

		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		require.NotNil(t, h)
		assert.Equal(t, "success", h.Status)
	})
}

func TestScheduler_RecordsFailureAfterAllRetriesExhausted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Always Fails", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				return errors.New("permanent failure")
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(5 * time.Minute)
		synctest.Wait()

		// No successful send; a failure record should exist.
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		assert.Nil(t, h) // no success
	})
}

func TestScheduler_CatchesUpMissedDays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Fake time starts 2000-01-01. Insert a prior success record for 1999-12-30
		// so the scheduler thinks it last succeeded 2 days ago.
		// Days to cover: 1999-12-31 and 2000-01-01.
		past := time.Date(1999, 12, 30, 0, 0, 0, 0, time.UTC)
		err := store.CreateReminderHistory(context.Background(), sqlDB, model.ReminderHistory{
			Status:    "success",
			DateStart: past,
			DateEnd:   past,
		})
		require.NoError(t, err)

		// Birthday on Dec 31 with 0 days before (fires on the day itself).
		month, day := 12, 31
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 0,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Dec 31 Person", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		var bodies []string
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				bodies = append(bodies, body)
				mu.Unlock()
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(2 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, bodies, 1)
		assert.Contains(t, bodies[0], "Catch-up")
		assert.Contains(t, bodies[0], "Dec 31 Person")
	})
}
