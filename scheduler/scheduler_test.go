package scheduler

import (
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestScheduler_ScheduleForward(t *testing.T) {
	user := &models.User{
		ID:            1,
		DailyCapacity: 4.0,
		WorkDays:      []int{1, 2, 3, 4, 5}, // Mon-Fri
	}

	tasks := []models.Task{
		{ID: 1, Title: "Task 1", HoursRequired: 6.0, Priority: 5},
	}

	s := NewScheduler(user, tasks)
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Monday

	result := s.Schedule(startDate)

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	if len(result.DaySchedules) != 2 {
		t.Errorf("Expected 2 days scheduled, got %d", len(result.DaySchedules))
	}

	// Day 1: 4 hours
	if result.DaySchedules[0].TotalHours != 4.0 {
		t.Errorf("Expected 4 hours on day 1, got %f", result.DaySchedules[0].TotalHours)
	}

	// Day 2: 2 hours
	if result.DaySchedules[1].TotalHours != 2.0 {
		t.Errorf("Expected 2 hours on day 2, got %f", result.DaySchedules[1].TotalHours)
	}
}

func TestScheduler_ScheduleBackward(t *testing.T) {
	user := &models.User{
		ID:            1,
		DailyCapacity: 8.0,
		WorkDays:      []int{1, 2, 3, 4, 5},
	}

	deadline := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC) // Friday
	tasks := []models.Task{
		{ID: 1, Title: "Deadline Task", HoursRequired: 12.0, Priority: 10, Deadline: &deadline},
	}

	s := NewScheduler(user, tasks)
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Monday

	result := s.Schedule(startDate)

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	// Backward planning should prefer days closer to deadline
	// Friday: 8 hours
	// Thursday: 4 hours

	foundFriday := false
	foundThursday := false
	for _, ds := range result.DaySchedules {
		if ds.Date.Equal(deadline) {
			foundFriday = true
			if ds.TotalHours != 8.0 {
				t.Errorf("Expected 8 hours on Friday, got %f", ds.TotalHours)
			}
		}
		if ds.Date.Equal(deadline.AddDate(0, 0, -1)) {
			foundThursday = true
			if ds.TotalHours != 4.0 {
				t.Errorf("Expected 4 hours on Thursday, got %f", ds.TotalHours)
			}
		}
	}

	if !foundFriday || !foundThursday {
		t.Errorf("Expected tasks on Friday and Thursday, found: Fri=%v, Thu=%v", foundFriday, foundThursday)
	}
}

func TestScheduler_PrioritySorting(t *testing.T) {
	user := &models.User{
		ID:            1,
		DailyCapacity: 8.0,
		WorkDays:      []int{1, 2, 3, 4, 5},
	}

	tasks := []models.Task{
		{ID: 1, Title: "Low Priority", HoursRequired: 4.0, Priority: 1},
		{ID: 2, Title: "High Priority", HoursRequired: 4.0, Priority: 10},
	}

	s := NewScheduler(user, tasks)
	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Monday

	result := s.Schedule(startDate)

	if len(result.DaySchedules) != 1 {
		t.Fatalf("Expected 1 day, got %d", len(result.DaySchedules))
	}

	day := result.DaySchedules[0]
	if len(day.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks in day, got %d", len(day.Tasks))
	}

	// High priority should be first in the list for the day (due to sorting before allocation)
	if day.Tasks[0].TaskID != 2 {
		t.Errorf("Expected High Priority task (ID 2) to be first, got ID %d", day.Tasks[0].TaskID)
	}
}
