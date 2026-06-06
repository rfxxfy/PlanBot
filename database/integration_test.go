package database

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func requireTestDB(t *testing.T) {
	t.Helper()
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST not set, skipping integration test")
	}
	if DB == nil {
		connStr := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_SSLMODE"),
		)
		var err error
		DB, err = sql.Open("postgres", connStr)
		if err != nil {
			t.Skipf("database unavailable: %v", err)
		}
		if err = DB.Ping(); err != nil {
			t.Skipf("database unavailable: %v", err)
		}
	}
	if _, err := DB.Exec(string(mustReadSchema(t))); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if err := EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
}

func mustReadSchema(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	return data
}

func TestGetOrCreateUser_Integration(t *testing.T) {
	requireTestDB(t)

	telegramID := time.Now().UnixNano()
	user, err := GetOrCreateUser(telegramID, "tester", "Test", "User")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	if user.TelegramID != telegramID {
		t.Errorf("telegram id = %d, want %d", user.TelegramID, telegramID)
	}

	again, err := GetOrCreateUser(telegramID, "tester", "Test", "User")
	if err != nil {
		t.Fatalf("second GetOrCreateUser: %v", err)
	}
	if again.ID != user.ID {
		t.Errorf("expected same user id %d, got %d", user.ID, again.ID)
	}

	t.Cleanup(func() {
		if _, err := DB.Exec("DELETE FROM users WHERE telegram_id = $1", telegramID); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
}

func TestCreateTaskAndSchedules_Integration(t *testing.T) {
	requireTestDB(t)

	telegramID := time.Now().UnixNano() + 1
	user, err := GetOrCreateUser(telegramID, "tasker", "Task", "Owner")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	t.Cleanup(func() {
		if _, err := DB.Exec("DELETE FROM users WHERE telegram_id = $1", telegramID); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	deadline := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	task := &models.Task{
		UserID:        user.ID,
		Title:         "Integration task",
		Description:   "created in test",
		HoursRequired: 2.5,
		Priority:      7,
		Deadline:      &deadline,
	}
	if err := CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID == 0 || task.Status == "" {
		t.Errorf("task not populated after create: %+v", task)
	}

	schedules := []models.DaySchedule{
		{
			Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Tasks: []models.ScheduledTaskInfo{
				{TaskID: task.ID, HoursAllocated: 1.5},
			},
		},
		{
			Date: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			Tasks: []models.ScheduledTaskInfo{
				{TaskID: task.ID, HoursAllocated: 1.0},
			},
		},
	}
	if err := SaveTaskSchedules(schedules); err != nil {
		t.Fatalf("SaveTaskSchedules: %v", err)
	}

	active, err := GetActiveTasks(user.ID)
	if err != nil {
		t.Fatalf("GetActiveTasks: %v", err)
	}
	found := false
	for _, at := range active {
		if at.ID == task.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created task not found among active tasks")
	}
}

func TestUpdateUserSettings_Integration(t *testing.T) {
	requireTestDB(t)

	telegramID := time.Now().UnixNano() + 2
	user, err := GetOrCreateUser(telegramID, "setter", "Set", "User")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	t.Cleanup(func() {
		if _, err := DB.Exec("DELETE FROM users WHERE telegram_id = $1", telegramID); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	if err := UpdateUserSettings(user.ID, 6, []int{1, 2, 3}, "10:00", "19:00"); err != nil {
		t.Fatalf("UpdateUserSettings: %v", err)
	}

	updated, err := GetOrCreateUser(telegramID, "setter", "Set", "User")
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.DailyCapacity != 6 || updated.WorkStart != "10:00" || updated.WorkEnd != "19:00" {
		t.Errorf("settings not updated: %+v", updated)
	}
}
