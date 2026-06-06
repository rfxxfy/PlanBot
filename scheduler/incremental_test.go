package scheduler

import (
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestScheduleTaskIntoExisting_FitsInGap(t *testing.T) {
	loc := time.UTC
	user := &models.User{
		ID:            1,
		DailyCapacity: 4,
		WorkDays:      []int{1, 2, 3, 4, 5},
		WorkStart:     "09:00",
		WorkEnd:       "13:00",
	}
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, loc)

	existing := []models.DaySchedule{
		{
			Date: startDate,
			Tasks: []models.ScheduledTaskInfo{
				{TaskID: 1, Title: "Existing", HoursAllocated: 2, Priority: 5},
			},
			TotalHours: 2,
		},
	}

	newTask := models.Task{
		ID:            2,
		Title:         "New",
		HoursRequired: 1,
		Priority:      3,
	}

	days, ok := ScheduleTaskIntoExisting(user, &newTask, existing, startDate, nil)
	if !ok {
		t.Fatal("expected task to be scheduled")
	}
	if len(days) != 1 {
		t.Fatalf("expected 1 day with new task, got %d", len(days))
	}
	if days[0].Tasks[0].TaskID != 2 || days[0].Tasks[0].HoursAllocated != 1 {
		t.Errorf("unexpected new task placement: %+v", days[0].Tasks)
	}
}

func TestScheduleTaskIntoExisting_ZeroHours(t *testing.T) {
	user := &models.User{ID: 1, DailyCapacity: 8, WorkDays: []int{1, 2, 3, 4, 5}}
	task := models.Task{ID: 1, HoursRequired: 0}
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

	_, ok := ScheduleTaskIntoExisting(user, &task, nil, start, nil)
	if ok {
		t.Error("expected failure for zero-hour task")
	}
}

func TestScheduleTaskIntoExisting_MissesDeadline(t *testing.T) {
	loc := time.UTC
	user := &models.User{
		ID:            1,
		DailyCapacity: 2,
		WorkDays:      []int{1},
		WorkStart:     "09:00",
		WorkEnd:       "11:00",
	}
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, loc) // Monday
	deadline := time.Date(2025, 1, 6, 0, 0, 0, 0, loc)

	newTask := models.Task{
		ID:            9,
		Title:         "Too big",
		HoursRequired: 10,
		Deadline:      &deadline,
	}

	_, ok := ScheduleTaskIntoExisting(user, &newTask, nil, startDate, nil)
	if ok {
		t.Error("expected failure when task does not fit before deadline")
	}
}
