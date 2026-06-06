package handlers

import (
	"context"
	"log"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/googlecal"
)

func (h *BotHandler) syncTaskCompletionToCalendar(userID, taskID int64) error {
	eventIDs, err := database.GetTaskCalendarEventIDs(userID, taskID)
	if err != nil || len(eventIDs) == 0 {
		return err
	}

	client, err := googlecal.ClientForUser(context.Background(), userID)
	if err != nil || client == nil {
		return err
	}

	for _, eventID := range eventIDs {
		if err := client.MarkTaskCompletedInCalendar(context.Background(), "primary", eventID); err != nil {
			log.Printf("calendar complete sync failed for event %s: %v", eventID, err)
		}
	}
	return nil
}

func (h *BotHandler) deleteTaskFromCalendar(userID, taskID int64) error {
	eventIDs, err := database.GetTaskCalendarEventIDs(userID, taskID)
	if err != nil || len(eventIDs) == 0 {
		return err
	}

	client, err := googlecal.ClientForUser(context.Background(), userID)
	if err != nil || client == nil {
		return err
	}

	for _, eventID := range eventIDs {
		if err := client.DeleteEventByID(context.Background(), "primary", eventID); err != nil {
			log.Printf("calendar delete sync failed for event %s: %v", eventID, err)
		}
	}
	return nil
}
