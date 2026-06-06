package googlecal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

// CalendarImportItem is an external calendar event that can be imported into bot tasks.
type CalendarImportItem struct {
	EventID   string
	Title     string
	Start     time.Time
	End       time.Time
	AllDay    bool
	SourceRaw *calendar.Event
}

// ListImportableEvents returns non-PlanBot events for import.
func (c *Client) ListImportableEvents(ctx context.Context, calendarID string, timeMin, timeMax time.Time, tz string) ([]CalendarImportItem, error) {
	if calendarID == "" {
		calendarID = calendarIDPrimary
	}
	call := c.svc.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		OrderBy("startTime").
		TimeMin(timeMin.Format(time.RFC3339)).
		TimeMax(timeMax.Format(time.RFC3339))
	if tz != "" {
		call = call.TimeZone(tz)
	}

	events, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("googlecal: list importable events: %w", err)
	}

	out := make([]CalendarImportItem, 0, len(events.Items))
	for _, ev := range events.Items {
		if ev.Status == "cancelled" || isPlanBotCalendarEvent(ev) {
			continue
		}
		loc := time.UTC
		if tz != "" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}
		start, end, allDay, ok := eventTimeRange(ev, loc)
		if !ok || !end.After(start) {
			continue
		}
		title := strings.TrimSpace(ev.Summary)
		if title == "" {
			title = "Импорт из календаря"
		}
		out = append(out, CalendarImportItem{
			EventID:   ev.Id,
			Title:     title,
			Start:     start,
			End:       end,
			AllDay:    allDay,
			SourceRaw: ev,
		})
	}
	return out, nil
}

// MarkTaskCompletedInCalendar prefixes task event title with check mark.
func (c *Client) MarkTaskCompletedInCalendar(ctx context.Context, calendarID, eventID string) error {
	if calendarID == "" {
		calendarID = calendarIDPrimary
	}
	ev, err := c.svc.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		if apiErr, ok := err.(*googleapi.Error); ok && (apiErr.Code == 404 || apiErr.Code == 410) {
			return nil
		}
		return err
	}
	if strings.HasPrefix(ev.Summary, "✅ ") {
		return nil
	}
	ev.Summary = "✅ " + strings.TrimSpace(strings.TrimPrefix(ev.Summary, "☐ "))
	_, err = c.svc.Events.Update(calendarID, eventID, ev).Context(ctx).Do()
	return err
}

// DeleteEventByID removes calendar event.
func (c *Client) DeleteEventByID(ctx context.Context, calendarID, eventID string) error {
	if calendarID == "" {
		calendarID = calendarIDPrimary
	}
	err := c.svc.Events.Delete(calendarID, eventID).Context(ctx).Do()
	if apiErr, ok := err.(*googleapi.Error); ok && (apiErr.Code == 404 || apiErr.Code == 410) {
		return nil
	}
	return err
}

func eventTimeRange(ev *calendar.Event, loc *time.Location) (start, end time.Time, allDay, ok bool) {
	if ev.Start != nil && ev.Start.Date != "" {
		var err error
		start, err = time.ParseInLocation("2006-01-02", ev.Start.Date, loc)
		if err != nil {
			return time.Time{}, time.Time{}, false, false
		}
		end := start.AddDate(0, 0, 1)
		if ev.End != nil && ev.End.Date != "" {
			if parsed, err := time.ParseInLocation("2006-01-02", ev.End.Date, loc); err == nil {
				end = parsed
			}
		}
		return start, end, true, true
	}
	if ev.Start == nil || ev.Start.DateTime == "" || ev.End == nil || ev.End.DateTime == "" {
		return time.Time{}, time.Time{}, false, false
	}
	var err error
	start, err = parseEventDateTime(ev.Start, loc)
	if err != nil {
		return time.Time{}, time.Time{}, false, false
	}
	end, err = parseEventDateTime(ev.End, loc)
	if err != nil {
		return time.Time{}, time.Time{}, false, false
	}
	return start, end, false, true
}
