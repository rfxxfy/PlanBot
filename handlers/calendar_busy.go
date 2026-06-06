package handlers

import (
	"context"
	"log"
	"time"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/googlecal"
	"github.com/adkhorst/planbot/models"
	"github.com/adkhorst/planbot/scheduler"
)

// clearPlanBotCalendar removes exported PlanBot events before a full rebuild
// so they do not block re-scheduling the same tasks.
func (h *BotHandler) clearPlanBotCalendar(user *models.User) {
	ctx := context.Background()
	client, err := googlecal.ClientForUser(ctx, user.ID)
	if err != nil {
		log.Printf("calendar clear: client: %v", err)
		return
	}
	if client == nil {
		return
	}
	if err := client.DeleteStoredEvents(ctx, "primary", user.ID); err != nil {
		log.Printf("calendar clear: %v", err)
		return
	}
	log.Printf("calendar clear: removed PlanBot events for user %d before rebuild", user.ID)
}

// fetchCalendarBusy loads busy intervals from Google Calendar when connected.
// forRebuild: skip PlanBot-tagged events and do not use stored PlanBot event IDs.
func (h *BotHandler) fetchCalendarBusy(user *models.User, startDate time.Time, forRebuild bool) []models.BusyInterval {
	ctx := context.Background()
	client, err := googlecal.ClientForUser(ctx, user.ID)
	if err != nil {
		log.Printf("calendar busy: client: %v", err)
		return nil
	}
	if client == nil {
		return nil
	}

	end := scheduler.HorizonEndDate(startDate)

	var parts [][]models.BusyInterval

	if apiBusy, err := client.FetchBusyIntervals(ctx, "primary", user, startDate, end, forRebuild); err != nil {
		log.Printf("calendar busy: fetch: %v", err)
	} else {
		parts = append(parts, apiBusy)
	}

	// Stored PlanBot exports are only used when inserting into an existing plan.
	if !forRebuild {
		if stored, err := database.GetStoredCalendarBusy(user.ID, startDate, end); err != nil {
			log.Printf("calendar busy: stored: %v", err)
		} else {
			parts = append(parts, stored)
		}
	}

	busy := scheduler.MergeBusyIntervals(parts...)
	log.Printf("calendar busy: %d intervals for user %d (rebuild=%v)", len(busy), user.ID, forRebuild)
	return busy
}
