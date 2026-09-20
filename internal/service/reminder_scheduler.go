package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/raddigo/raddigo/internal/repository"
)

// istLocation is a fixed +05:30 offset (India has no daylight saving), so
// booking slot_date/slot_start_time can be compared against wall-clock time
// without depending on the IANA tzdata database being installed.
var istLocation = time.FixedZone("IST", 5*3600+30*60)

// ReminderScheduler polls for accepted bookings whose pickup slot is coming up
// and sends a one-time reminder notification.
type ReminderScheduler struct {
	bookings      repository.BookingRepository
	notifications *NotificationService
	interval      time.Duration
	leadMinutes   int
	logger        *slog.Logger
	now           func() time.Time
}

// NewReminderScheduler creates a ReminderScheduler. leadMinutes is how long
// before the scheduled pickup the reminder should fire.
func NewReminderScheduler(bookings repository.BookingRepository, notifications *NotificationService, interval time.Duration, leadMinutes int, logger *slog.Logger) *ReminderScheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	if leadMinutes <= 0 {
		leadMinutes = 30
	}
	return &ReminderScheduler{
		bookings:      bookings,
		notifications: notifications,
		interval:      interval,
		leadMinutes:   leadMinutes,
		logger:        logger,
		now:           time.Now,
	}
}

// Run polls on a ticker until ctx is cancelled.
func (s *ReminderScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick checks for bookings whose pickup falls inside the reminder window and
// sends (and claims) the reminder for each one found.
func (s *ReminderScheduler) tick(ctx context.Context) {
	// The window is padded by the poll interval on both sides so a booking is
	// never missed between ticks; ClaimReminder guarantees it is still only
	// ever sent once.
	pad := s.interval
	if pad < time.Minute {
		pad = time.Minute
	}
	target := s.now().In(istLocation).Add(time.Duration(s.leadMinutes) * time.Minute)
	windowStart := target.Add(-pad)
	windowEnd := target.Add(pad)

	for _, w := range splitByDate(windowStart, windowEnd) {
		bookings, err := s.bookings.ListPickupsDueForReminder(ctx, w.date, w.fromTime, w.toTime)
		if err != nil {
			s.logger.Error("list pickups due for reminder", "error", err)
			continue
		}
		for _, booking := range bookings {
			claimed, err := s.bookings.ClaimReminder(ctx, booking.ID, s.now())
			if err != nil {
				s.logger.Error("claim pickup reminder", "error", err, "booking_id", booking.ID)
				continue
			}
			if !claimed {
				continue
			}
			s.notifications.PickupReminder(ctx, booking)
		}
	}
}

type dateWindow struct {
	date     string
	fromTime string
	toTime   string
}

// splitByDate breaks a [from, to] time range into per-calendar-day windows, so
// a reminder window that straddles midnight still queries each day correctly.
func splitByDate(from, to time.Time) []dateWindow {
	const dateFormat = "2006-01-02"
	const timeFormat = "15:04"

	if from.Format(dateFormat) == to.Format(dateFormat) {
		return []dateWindow{{
			date:     from.Format(dateFormat),
			fromTime: from.Format(timeFormat),
			toTime:   to.Format(timeFormat),
		}}
	}

	endOfFromDay := time.Date(from.Year(), from.Month(), from.Day(), 23, 59, 0, 0, from.Location())
	startOfToDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
	return []dateWindow{
		{date: from.Format(dateFormat), fromTime: from.Format(timeFormat), toTime: endOfFromDay.Format(timeFormat)},
		{date: to.Format(dateFormat), fromTime: startOfToDay.Format(timeFormat), toTime: to.Format(timeFormat)},
	}
}
