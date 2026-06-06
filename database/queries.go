package database

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/lib/pq"
	"golang.org/x/oauth2"

	"github.com/adkhorst/planbot/models"
)

// GetOrCreateUser gets existing user or creates a new one
func GetOrCreateUser(telegramID int64, username, firstName, lastName string) (*models.User, error) {
	user := &models.User{}

	// Try to get existing user
	query := `SELECT id, telegram_id, username, first_name, last_name, time_zone, work_start, work_end, daily_capacity, work_days, created_at, updated_at
			  FROM users WHERE telegram_id = $1`

	var workDays pq.Int64Array
	var usernameNull, fName, lName sql.NullString
	err := DB.QueryRow(query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&usernameNull,
		&fName,
		&lName,
		&user.TimeZone,
		&user.WorkStart,
		&user.WorkEnd,
		&user.DailyCapacity,
		&workDays,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create new user
		insertQuery := `INSERT INTO users (telegram_id, username, first_name, last_name)
						VALUES ($1, $2, $3, $4)
						RETURNING id, telegram_id, username, first_name, last_name, time_zone, work_start, work_end, daily_capacity, work_days, created_at, updated_at`

		err = DB.QueryRow(insertQuery, telegramID, username, firstName, lastName).Scan(
			&user.ID,
			&user.TelegramID,
			&usernameNull,
			&fName,
			&lName,
			&user.TimeZone,
			&user.WorkStart,
			&user.WorkEnd,
			&user.DailyCapacity,
			&workDays,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user.Username = usernameNull.String
	user.FirstName = fName.String
	user.LastName = lName.String

	// Convert pq.Int64Array to []int
	user.WorkDays = make([]int, len(workDays))
	for i, v := range workDays {
		user.WorkDays[i] = int(v)
	}

	return user, nil
}

// UpdateUserSettings updates user's daily capacity, work days and work hours
func UpdateUserSettings(userID int64, dailyCapacity float64, workDays []int, workStart, workEnd string) error {
	// Convert []int to pq.Int64Array
	workDaysArray := make(pq.Int64Array, len(workDays))
	for i, v := range workDays {
		workDaysArray[i] = int64(v)
	}

	query := `UPDATE users SET daily_capacity = $1, work_days = $2, work_start = $3, work_end = $4, updated_at = NOW()
			  WHERE id = $5`

	_, err := DB.Exec(query, dailyCapacity, workDaysArray, workStart, workEnd, userID)
	if err != nil {
		return fmt.Errorf("failed to update user settings: %w", err)
	}

	return nil
}

// UpdateUserTimeZone updates user's time zone
func UpdateUserTimeZone(userID int64, timeZone string) error {
	query := `UPDATE users SET time_zone = $1, updated_at = NOW()
			  WHERE id = $2`

	_, err := DB.Exec(query, timeZone, userID)
	if err != nil {
		return fmt.Errorf("failed to update user time zone: %w", err)
	}

	return nil
}

// CreateTask creates a new task
func CreateTask(task *models.Task) error {
	query := `INSERT INTO tasks (user_id, title, description, hours_required, priority, deadline)
			  VALUES ($1, $2, $3, $4, $5, $6)
			  RETURNING id, created_at, updated_at, status`

	err := DB.QueryRow(query,
		task.UserID,
		task.Title,
		task.Description,
		task.HoursRequired,
		task.Priority,
		task.Deadline,
	).Scan(&task.ID, &task.CreatedAt, &task.UpdatedAt, &task.Status)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetUserTasks retrieves all tasks for a user
func GetUserTasks(userID int64) ([]models.Task, error) {
	query := `SELECT id, user_id, title, description, hours_required, priority, status, deadline, 
			  created_at, updated_at, completed_at
			  FROM tasks WHERE user_id = $1 ORDER BY priority DESC, deadline ASC NULLS LAST`

	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer closeRows(rows)

	tasks := []models.Task{}
	for rows.Next() {
		task := models.Task{}
		var desc sql.NullString
		err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&desc,
			&task.HoursRequired,
			&task.Priority,
			&task.Status,
			&task.Deadline,
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.CompletedAt,
		)
		task.Description = desc.String
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetPendingTasks retrieves all pending tasks for a user
func GetPendingTasks(userID int64) ([]models.Task, error) {
	query := `SELECT id, user_id, title, description, hours_required, priority, status, deadline, 
			  created_at, updated_at, completed_at
			  FROM tasks WHERE user_id = $1 AND status = 'pending' 
			  ORDER BY priority DESC, deadline ASC NULLS LAST`

	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}
	defer closeRows(rows)

	tasks := []models.Task{}
	for rows.Next() {
		task := models.Task{}
		var desc sql.NullString
		err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&desc,
			&task.HoursRequired,
			&task.Priority,
			&task.Status,
			&task.Deadline,
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.CompletedAt,
		)
		task.Description = desc.String
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetActiveTasks returns tasks that should participate in (re)planning.
// "Hard" rescheduling treats all non-completed / non-cancelled tasks as current.
func GetActiveTasks(userID int64) ([]models.Task, error) {
	query := `
		SELECT id, user_id, title, description, hours_required, priority, status, deadline,
			   created_at, updated_at, completed_at
		FROM tasks
		WHERE user_id = $1 AND status NOT IN ('completed', 'cancelled')
		ORDER BY priority DESC, deadline ASC NULLS LAST
	`

	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active tasks: %w", err)
	}
	defer closeRows(rows)

	tasks := []models.Task{}
	for rows.Next() {
		task := models.Task{}
		var desc sql.NullString
		err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&desc,
			&task.HoursRequired,
			&task.Priority,
			&task.Status,
			&task.Deadline,
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.CompletedAt,
		)
		task.Description = desc.String
		if err != nil {
			return nil, fmt.Errorf("failed to scan active task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// UpdateTaskStatus updates the status of a task
func UpdateTaskStatus(taskID int64, status string) error {
	query := `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`

	_, err := DB.Exec(query, status, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// CompleteTask marks a task as completed
func CompleteTask(taskID int64) error {
	query := `UPDATE tasks SET status = 'completed', completed_at = NOW(), updated_at = NOW() 
			  WHERE id = $1`

	_, err := DB.Exec(query, taskID)
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	return nil
}

// DeleteTask deletes a task
func DeleteTask(taskID int64) error {
	query := `DELETE FROM tasks WHERE id = $1`

	_, err := DB.Exec(query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	return nil
}

// ClearTaskSchedules removes all schedules for given tasks
func ClearTaskSchedules(taskIDs []int64) error {
	if len(taskIDs) == 0 {
		return nil
	}

	query := `DELETE FROM task_schedules WHERE task_id = ANY($1)`
	_, err := DB.Exec(query, pq.Array(taskIDs))
	if err != nil {
		return fmt.Errorf("failed to clear task schedules: %w", err)
	}

	return nil
}

// SaveTaskSchedules saves schedule entries to database
func SaveTaskSchedules(schedules []models.DaySchedule) error {
	if len(schedules) == 0 {
		return nil
	}

	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackTx(tx)

	// Prepare insert statement
	stmt, err := tx.Prepare(`INSERT INTO task_schedules (task_id, scheduled_date, hours_allocated)
							 VALUES ($1, $2, $3)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer closeStmt(stmt)

	// Insert all schedules
	taskIDs := make(map[int64]bool)
	for _, daySchedule := range schedules {
		for _, taskInfo := range daySchedule.Tasks {
			_, err := stmt.Exec(taskInfo.TaskID, daySchedule.Date, taskInfo.HoursAllocated)
			if err != nil {
				return fmt.Errorf("failed to insert schedule: %w", err)
			}
			taskIDs[taskInfo.TaskID] = true
		}
	}

	// Update task status to 'scheduled'
	for taskID := range taskIDs {
		_, err := tx.Exec(`UPDATE tasks SET status = 'scheduled', updated_at = NOW() WHERE id = $1`, taskID)
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTaskByIDForUser returns a task if it belongs to the user.
func GetTaskByIDForUser(taskID, userID int64) (*models.Task, error) {
	query := `SELECT id, user_id, title, description, hours_required, priority, status, deadline, created_at, updated_at, completed_at
			  FROM tasks WHERE id = $1 AND user_id = $2`

	task := &models.Task{}
	err := DB.QueryRow(query, taskID, userID).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.HoursRequired,
		&task.Priority,
		&task.Status,
		&task.Deadline,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return task, nil
}

// UserHasScheduledTasks reports whether the user already has entries in task_schedules.
func UserHasScheduledTasks(userID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM task_schedules ts
		JOIN tasks t ON t.id = ts.task_id
		WHERE t.user_id = $1 AND t.status NOT IN ('completed', 'cancelled')
	)`
	err := DB.QueryRow(query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check scheduled tasks: %w", err)
	}
	return exists, nil
}

// GetAllUserSchedulesFrom returns all saved day schedules from the given date onward.
func GetAllUserSchedulesFrom(userID int64, fromDate time.Time) ([]models.DaySchedule, error) {
	horizonEnd := fromDate.AddDate(0, 0, 366)
	schedules, err := GetScheduleForDateRange(userID, fromDate, horizonEnd)
	if err != nil {
		return nil, err
	}
	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].Date.Before(schedules[j].Date)
	})
	return schedules, nil
}

// GetScheduleForDateRange retrieves schedule for a date range
func GetScheduleForDateRange(userID int64, startDate, endDate time.Time) ([]models.DaySchedule, error) {
	query := `SELECT ts.scheduled_date, ts.task_id, t.title, ts.hours_allocated, t.priority, t.deadline
			  FROM task_schedules ts
			  JOIN tasks t ON ts.task_id = t.id
			  WHERE t.user_id = $1 AND ts.scheduled_date >= $2 AND ts.scheduled_date <= $3
			  ORDER BY ts.scheduled_date, t.priority DESC`

	rows, err := DB.Query(query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query schedules: %w", err)
	}
	defer closeRows(rows)

	// Group by date
	scheduleMap := make(map[string]*models.DaySchedule)

	for rows.Next() {
		var scheduledDate time.Time
		taskInfo := models.ScheduledTaskInfo{}

		err := rows.Scan(
			&scheduledDate,
			&taskInfo.TaskID,
			&taskInfo.Title,
			&taskInfo.HoursAllocated,
			&taskInfo.Priority,
			&taskInfo.Deadline,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}

		dateKey := scheduledDate.Format("2006-01-02")
		daySchedule, exists := scheduleMap[dateKey]
		if !exists {
			daySchedule = &models.DaySchedule{
				Date:       scheduledDate,
				Tasks:      []models.ScheduledTaskInfo{},
				TotalHours: 0,
			}
			scheduleMap[dateKey] = daySchedule
		}

		daySchedule.Tasks = append(daySchedule.Tasks, taskInfo)
		daySchedule.TotalHours += taskInfo.HoursAllocated
	}

	// Convert map to slice
	schedules := make([]models.DaySchedule, 0, len(scheduleMap))
	for _, schedule := range scheduleMap {
		schedules = append(schedules, *schedule)
	}

	return schedules, nil
}

// GetGoogleToken returns stored Google OAuth token for user.
func GetGoogleToken(userID int64) (*models.GoogleToken, error) {
	query := `SELECT user_id, access_token, refresh_token, expiry, created_at, updated_at
	          FROM user_google_tokens WHERE user_id = $1`

	tok := &models.GoogleToken{}
	err := DB.QueryRow(query, userID).Scan(
		&tok.UserID,
		&tok.AccessToken,
		&tok.RefreshToken,
		&tok.Expiry,
		&tok.CreatedAt,
		&tok.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get google token: %w", err)
	}
	return tok, nil
}

// SaveGoogleToken upserts user's Google OAuth token.
func SaveGoogleToken(userID int64, token *oauth2.Token) error {
	if token == nil {
		return fmt.Errorf("nil token")
	}
	query := `
		INSERT INTO user_google_tokens (user_id, access_token, refresh_token, expiry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    expiry = EXCLUDED.expiry,
		    updated_at = NOW()
	`

	_, err := DB.Exec(query, userID, token.AccessToken, token.RefreshToken, token.Expiry)
	if err != nil {
		return fmt.Errorf("failed to save google token: %w", err)
	}
	return nil
}

// GetGoogleCalendarEventIDs returns stored Google event IDs for a user.
func GetGoogleCalendarEventIDs(userID int64) ([]string, error) {
	rows, err := DB.Query(`SELECT google_event_id FROM google_calendar_events WHERE user_id = $1 AND source = 'planbot'`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list google events: %w", err)
	}
	defer closeRows(rows)

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan google event id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ClearGoogleCalendarEvents removes stored event metadata for a user.
func ClearGoogleCalendarEvents(userID int64) error {
	_, err := DB.Exec(`DELETE FROM google_calendar_events WHERE user_id = $1 AND source = 'planbot'`, userID)
	if err != nil {
		return fmt.Errorf("failed to clear google events: %w", err)
	}
	return nil
}

// SaveGoogleCalendarEvents stores exported Google Calendar event IDs.
func SaveGoogleCalendarEvents(userID int64, events []models.GoogleCalendarEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackTx(tx)

	stmt, err := tx.Prepare(`INSERT INTO google_calendar_events (user_id, google_event_id, task_id, source, start_time, end_time)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, google_event_id) DO UPDATE
		SET task_id = EXCLUDED.task_id,
		    source = EXCLUDED.source,
		    start_time = EXCLUDED.start_time,
		    end_time = EXCLUDED.end_time`)
	if err != nil {
		return fmt.Errorf("failed to prepare google events insert: %w", err)
	}
	defer closeStmt(stmt)

	for i := range events {
		ev := events[i]
		source := ev.Source
		if source == "" {
			source = "planbot"
		}
		_, err := stmt.Exec(userID, ev.GoogleEventID, ev.TaskID, source, ev.StartTime, ev.EndTime)

		if err != nil {
			return fmt.Errorf("failed to insert google event: %w", err)
		}
	}

	return tx.Commit()
}
