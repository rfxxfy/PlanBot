package googlecal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/adkhorst/planbot/models"
)

// FetchBusyIntervals returns occupied time from Google Calendar.
// If excludePlanBot is true, events created by PlanBot are skipped (for full rebuild).
func (c *Client) FetchBusyIntervals(ctx context.Context, calendarID string, user *models.User, timeMin, timeMax time.Time, excludePlanBot bool) ([]models.BusyInterval, error) {
	if calendarID == "" {
		calendarID = calendarIDPrimary
	}

	loc := userLocation(user)
	timeMin = timeMin.In(loc)
	timeMax = timeMax.In(loc)

	call := c.svc.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		OrderBy("startTime").
		TimeMin(timeMin.Format(time.RFC3339)).
		TimeMax(timeMax.Format(time.RFC3339))

	if user.TimeZone != "" {
		call = call.TimeZone(user.TimeZone)
	}

	events, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("googlecal: list events: %w", err)
	}

	var busy []models.BusyInterval
	for _, ev := range events.Items {
		if ev.Status == "cancelled" {
			continue
		}
		if excludePlanBot && isPlanBotCalendarEvent(ev) {
			continue
		}

		interval, ok := eventToBusyInterval(ev, loc, user)
		if !ok {
			continue
		}
		busy = append(busy, interval)
	}

	return busy, nil
}

func eventToBusyInterval(ev *calendar.Event, defaultLoc *time.Location, user *models.User) (models.BusyInterval, bool) {
	summary := ev.Summary
	if summary == "" {
		summary = "Занято"
	}

	if ev.Start != nil && ev.Start.Date != "" {
		startDay, err := time.ParseInLocation("2006-01-02", ev.Start.Date, defaultLoc)
		if err != nil {
			return models.BusyInterval{}, false
		}
		// Block only working hours on all-day events, not the full 24h.
		return allDayBusyInterval(user, startDay, summary), true
	}

	if ev.Start == nil || ev.Start.DateTime == "" || ev.End == nil || ev.End.DateTime == "" {
		return models.BusyInterval{}, false
	}

	start, err := parseEventDateTime(ev.Start, defaultLoc)
	if err != nil {
		return models.BusyInterval{}, false
	}
	end, err := parseEventDateTime(ev.End, defaultLoc)
	if err != nil {
		return models.BusyInterval{}, false
	}
	if !end.After(start) {
		return models.BusyInterval{}, false
	}

	return models.BusyInterval{
		Start:   start,
		End:     end,
		Summary: summary,
		Source:  "calendar",
		AllDay:  false,
	}, true
}

func parseEventDateTime(dt *calendar.EventDateTime, defaultLoc *time.Location) (time.Time, error) {
	loc := defaultLoc
	if dt.TimeZone != "" {
		if l, err := time.LoadLocation(dt.TimeZone); err == nil {
			loc = l
		}
	}
	if dt.DateTime != "" {
		t, err := time.ParseInLocation(time.RFC3339, dt.DateTime, loc)
		if err != nil {
			t, err = time.ParseInLocation("2006-01-02T15:04:05", dt.DateTime, loc)
			if err != nil {
				return time.Time{}, err
			}
		}
		return t.In(defaultLoc), nil
	}
	if dt.Date != "" {
		return time.ParseInLocation("2006-01-02", dt.Date, loc)
	}
	return time.Time{}, fmt.Errorf("empty event datetime")
}

func allDayBusyInterval(user *models.User, day time.Time, summary string) models.BusyInterval {
	workStart := user.WorkStart
	workEnd := user.WorkEnd
	if workStart == "" {
		workStart = "09:00"
	}
	if workEnd == "" {
		workEnd = "18:00"
	}
	loc := day.Location()
	startClock, err1 := time.ParseInLocation("15:04", workStart, loc)
	endClock, err2 := time.ParseInLocation("15:04", workEnd, loc)
	if err1 != nil || err2 != nil || !endClock.After(startClock) {
		return models.BusyInterval{
			Start:   day,
			End:     day.AddDate(0, 0, 1),
			Summary: summary,
			Source:  "calendar",
			AllDay:  true,
		}
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), startClock.Hour(), startClock.Minute(), 0, 0, loc)
	end := time.Date(day.Year(), day.Month(), day.Day(), endClock.Hour(), endClock.Minute(), 0, 0, loc)
	return models.BusyInterval{
		Start:   start,
		End:     end,
		Summary: summary,
		Source:  "calendar",
		AllDay:  true,
	}
}

func isPlanBotCalendarEvent(ev *calendar.Event) bool {
	if ev.ExtendedProperties != nil && ev.ExtendedProperties.Private != nil {
		if ev.ExtendedProperties.Private["planbot"] == "1" {
			return true
		}
	}
	return strings.HasPrefix(ev.Description, "PlanBot")
}

func userLocation(user *models.User) *time.Location {
	tz := user.TimeZone
	if tz == "" {
		tz = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}
