package googlecal

import (
	"context"
	"fmt"
	"strings"

	"github.com/adkhorst/planbot/models"
	"google.golang.org/api/calendar/v3"
)

// ExportSlotAllocations creates timed calendar events and returns saved event metadata.
func (c *Client) ExportSlotAllocations(ctx context.Context, calendarID string, user *models.User, allocations []models.SlotAllocation) ([]models.GoogleCalendarEvent, error) {
	if calendarID == "" {
		calendarID = "primary"
	}
	if len(allocations) == 0 {
		return nil, nil
	}

	tz := user.TimeZone
	if tz == "" {
		tz = "Europe/Moscow"
	}

	var records []models.GoogleCalendarEvent

	for _, alloc := range allocations {
		summary := alloc.Title
		if summary == "" {
			summary = fmt.Sprintf("Задача #%d", alloc.TaskID)
		}
		if !strings.HasPrefix(summary, "☐ ") && !strings.HasPrefix(summary, "✅ ") {
			summary = "☐ " + summary
		}

		duration := alloc.End.Sub(alloc.Start).Hours()
		description := fmt.Sprintf("PlanBot\nДлительность: %.1f ч\nПриоритет: %d", duration, alloc.Priority)
		if alloc.Deadline != nil {
			description += fmt.Sprintf("\nДедлайн: %s", alloc.Deadline.Format("02.01.2006"))
		}

		ev := &calendar.Event{
			Summary:     summary,
			Description: description,
			Start: &calendar.EventDateTime{
				DateTime: alloc.Start.Format("2006-01-02T15:04:05"),
				TimeZone: tz,
			},
			End: &calendar.EventDateTime{
				DateTime: alloc.End.Format("2006-01-02T15:04:05"),
				TimeZone: tz,
			},
			ExtendedProperties: &calendar.EventExtendedProperties{
				Private: map[string]string{
					"planbot": "1",
					"task_id": fmt.Sprintf("%d", alloc.TaskID),
				},
			},
		}

		created, err := c.svc.Events.Insert(calendarID, ev).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("googlecal: insert timed event for %s (%s–%s): %w",
				summary, alloc.Start.Format("15:04"), alloc.End.Format("15:04"), err)
		}

		records = append(records, models.GoogleCalendarEvent{
			UserID:        user.ID,
			GoogleEventID: created.Id,
			TaskID:        alloc.TaskID,
			Source:        "planbot",
			StartTime:     alloc.Start,
			EndTime:       alloc.End,
		})
	}

	return records, nil
}
