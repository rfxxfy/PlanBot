package googlecal

import (
	"context"
	"log"

	"google.golang.org/api/googleapi"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/models"
)

const calendarIDPrimary = "primary"

// SyncUserSchedule writes the full PlanBot schedule to Google Calendar (export only).
// Call clearPlanBotCalendar / DeleteStoredEvents before planning on full rebuild.
func SyncUserSchedule(ctx context.Context, c *Client, user *models.User, allocations []models.SlotAllocation) error {
	if len(allocations) == 0 {
		return nil
	}

	records, err := c.ExportSlotAllocations(ctx, calendarIDPrimary, user, allocations)
	if err != nil {
		return err
	}
	return database.SaveGoogleCalendarEvents(user.ID, records)
}

// AppendScheduleEvents adds new timed events without removing existing PlanBot events.
func AppendScheduleEvents(ctx context.Context, c *Client, user *models.User, allocations []models.SlotAllocation) error {
	if len(allocations) == 0 {
		return nil
	}
	records, err := c.ExportSlotAllocations(ctx, calendarIDPrimary, user, allocations)
	if err != nil {
		return err
	}
	return database.SaveGoogleCalendarEvents(user.ID, records)
}

// DeleteStoredEvents removes PlanBot events from Google Calendar using IDs saved in the database.
func (c *Client) DeleteStoredEvents(ctx context.Context, calendarID string, userID int64) error {
	if calendarID == "" {
		calendarID = calendarIDPrimary
	}

	eventIDs, err := database.GetGoogleCalendarEventIDs(userID)
	if err != nil {
		return err
	}

	for _, eventID := range eventIDs {
		if err := c.svc.Events.Delete(calendarID, eventID).Context(ctx).Do(); err != nil {
			if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 410 {
				continue // already removed in Google Calendar
			}
			log.Printf("googlecal: delete event %s for user %d: %v", eventID, userID, err)
		}
	}

	return database.ClearGoogleCalendarEvents(userID)
}

// TrySyncUserSchedule runs calendar sync when Google is connected; logs errors without failing scheduling.
func TrySyncUserSchedule(ctx context.Context, user *models.User, allocations []models.SlotAllocation) {
	if len(allocations) == 0 {
		return
	}

	client, err := ClientForUser(ctx, user.ID)
	if err != nil || client == nil {
		return
	}

	if err := SyncUserSchedule(ctx, client, user, allocations); err != nil {
		log.Printf("google calendar sync: %v", err)
	}
}
