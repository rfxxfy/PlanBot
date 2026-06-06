package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/adkhorst/planbot/models"
)

// GetStoredCalendarBusy returns busy intervals from PlanBot events still tracked in DB.
// Used as a fallback when Google API is slow or to reinforce calendar blocks.
func GetStoredCalendarBusy(userID int64, from, to time.Time) ([]models.BusyInterval, error) {
	query := `SELECT start_time, end_time, COALESCE(t.title, '') 
		FROM google_calendar_events g
		LEFT JOIN tasks t ON t.id = g.task_id
		WHERE g.user_id = $1 AND g.source = 'planbot' AND g.end_time > $2 AND g.start_time < $3
		ORDER BY g.start_time`

	rows, err := DB.Query(query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query stored calendar events: %w", err)
	}
	defer rows.Close()

	var busy []models.BusyInterval
	for rows.Next() {
		var start, end time.Time
		var title string
		if err := rows.Scan(&start, &end, &title); err != nil {
			return nil, fmt.Errorf("failed to scan stored calendar event: %w", err)
		}
		if !end.After(start) {
			continue
		}
		if title == "" {
			title = "Календарь"
		}
		busy = append(busy, models.BusyInterval{
			Start:   start,
			End:     end,
			Summary: title,
			Source:  "calendar",
		})
	}
	return busy, rows.Err()
}

// GetTaskCalendarEventIDs returns all linked calendar event IDs for a task.
func GetTaskCalendarEventIDs(userID, taskID int64) ([]string, error) {
	rows, err := DB.Query(`SELECT google_event_id FROM google_calendar_events WHERE user_id = $1 AND task_id = $2`, userID, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query task calendar events: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan event id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteTaskCalendarLinks removes calendar links for a task.
func DeleteTaskCalendarLinks(userID, taskID int64) error {
	_, err := DB.Exec(`DELETE FROM google_calendar_events WHERE user_id = $1 AND task_id = $2`, userID, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task calendar links: %w", err)
	}
	return nil
}

// GetTaskIDByGoogleEventID returns linked task id for calendar event if exists.
func GetTaskIDByGoogleEventID(userID int64, googleEventID string) (*int64, error) {
	var taskID sql.NullInt64
	err := DB.QueryRow(`SELECT task_id FROM google_calendar_events WHERE user_id = $1 AND google_event_id = $2 LIMIT 1`, userID, googleEventID).Scan(&taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task by google event id: %w", err)
	}
	if !taskID.Valid {
		return nil, nil
	}
	id := taskID.Int64
	return &id, nil
}

// SaveImportedCalendarLink stores an external calendar event -> task mapping.
func SaveImportedCalendarLink(userID int64, googleEventID string, taskID int64, start, end time.Time) error {
	_, err := DB.Exec(`INSERT INTO google_calendar_events (user_id, google_event_id, task_id, source, start_time, end_time)
		VALUES ($1, $2, $3, 'imported', $4, $5)
		ON CONFLICT (user_id, google_event_id) DO UPDATE
		SET task_id = EXCLUDED.task_id,
		    source = EXCLUDED.source,
		    start_time = EXCLUDED.start_time,
		    end_time = EXCLUDED.end_time`,
		userID, googleEventID, taskID, start, end)
	if err != nil {
		return fmt.Errorf("failed to save imported calendar link: %w", err)
	}
	return nil
}
