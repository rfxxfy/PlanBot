package scheduler

import (
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestMergeSlotAllocations_AdjacentSameTask(t *testing.T) {
	loc := time.UTC
	start := time.Date(2025, 1, 6, 9, 0, 0, 0, loc)
	allocations := []models.SlotAllocation{
		{TaskID: 1, Title: "A", Start: start, End: start.Add(time.Hour)},
		{TaskID: 1, Title: "A", Start: start.Add(time.Hour), End: start.Add(2 * time.Hour)},
		{TaskID: 2, Title: "B", Start: start.Add(2 * time.Hour), End: start.Add(3 * time.Hour)},
	}

	merged := MergeSlotAllocations(allocations)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged blocks, got %d", len(merged))
	}
	if merged[0].End.Sub(merged[0].Start).Hours() != 2 {
		t.Errorf("task 1 block should be 2h, got %f", merged[0].End.Sub(merged[0].Start).Hours())
	}
}

func TestMergeSlotAllocations_Empty(t *testing.T) {
	if got := MergeSlotAllocations(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestPlanTimeAllocations_WithCalendarBusy(t *testing.T) {
	loc := time.UTC
	user := &models.User{
		ID:            1,
		DailyCapacity: 4,
		WorkDays:      []int{1, 2, 3, 4, 5},
		WorkStart:     "09:00",
		WorkEnd:       "13:00",
	}

	startDate := time.Date(2025, 1, 6, 0, 0, 0, 0, loc)
	daySchedules := []models.DaySchedule{
		{
			Date: startDate,
			Tasks: []models.ScheduledTaskInfo{
				{TaskID: 1, Title: "Report", HoursAllocated: 2, Priority: 5},
			},
			TotalHours: 2,
		},
	}

	busy := []models.BusyInterval{
		{
			Start: time.Date(2025, 1, 6, 9, 0, 0, 0, loc),
			End:   time.Date(2025, 1, 6, 10, 0, 0, 0, loc),
		},
	}

	allocations := PlanTimeAllocations(user, daySchedules, startDate, busy)
	if len(allocations) == 0 {
		t.Fatal("expected at least one allocation")
	}

	// First free slot after 09-10 busy block is 10:00
	if allocations[0].Start.Hour() != 10 {
		t.Errorf("expected allocation to start at 10:00, got %v", allocations[0].Start)
	}
}
