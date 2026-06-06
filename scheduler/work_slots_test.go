package scheduler

import (
	"os"
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestBlockSlotsFromBusy_ReducesFreeHours(t *testing.T) {
	loc := time.UTC
	day := time.Date(2025, 1, 6, 0, 0, 0, 0, loc) // Monday
	user := &models.User{
		ID:            1,
		DailyCapacity: 8,
		WorkDays:      []int{1, 2, 3, 4, 5},
		WorkStart:     "09:00",
		WorkEnd:       "12:00", // 3 hours total in 1h slots
	}

	slots := BuildWorkSlots(user, day, nil)
	if len(slots) == 0 {
		t.Fatal("expected work slots")
	}

	busy := []models.BusyInterval{
		{
			Start: time.Date(2025, 1, 6, 9, 0, 0, 0, loc),
			End:   time.Date(2025, 1, 6, 10, 0, 0, 0, loc),
		},
	}
	BlockSlotsFromBusy(slots, busy)

	free := FreeHoursOnDate(slots, "2025-01-06")
	if free != 2.0 {
		t.Errorf("expected 2 free hours after blocking 1h, got %f", free)
	}
	if slots[0].Source != "calendar" {
		t.Errorf("expected calendar source on blocked slot, got %q", slots[0].Source)
	}
}

func TestFreeHoursOnDate_UnknownDate(t *testing.T) {
	slots := []models.TimeSlot{
		{
			Date:           time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			CapacityHours:  1,
			AllocatedHours: 0,
		},
	}
	if got := FreeHoursOnDate(slots, "2099-01-01"); got != 0 {
		t.Errorf("expected 0 free hours for unknown date, got %f", got)
	}
}

func TestPlanningHorizonDays_FromEnv(t *testing.T) {
	t.Setenv("PLANNING_HORIZON_DAYS", "14")
	if got := PlanningHorizonDays(); got != 14 {
		t.Errorf("expected horizon 14, got %d", got)
	}
}

func TestPlanningHorizonDays_InvalidEnvFallsBack(t *testing.T) {
	t.Setenv("PLANNING_HORIZON_DAYS", "not-a-number")
	if got := PlanningHorizonDays(); got != 365 {
		t.Errorf("expected default horizon 365, got %d", got)
	}
	os.Unsetenv("PLANNING_HORIZON_DAYS")
}

func TestHorizonEndDate(t *testing.T) {
	t.Setenv("PLANNING_HORIZON_DAYS", "7")
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := HorizonEndDate(start)
	want := start.AddDate(0, 0, 7)
	if !end.Equal(want) {
		t.Errorf("HorizonEndDate() = %v, want %v", end, want)
	}
	os.Unsetenv("PLANNING_HORIZON_DAYS")
}

func TestSlotScheduler_BuildDailySlots_SkipsWeekend(t *testing.T) {
	t.Setenv("PLANNING_HORIZON_DAYS", "3")
	t.Cleanup(func() { os.Unsetenv("PLANNING_HORIZON_DAYS") })

	user := &models.User{
		ID:        1,
		WorkDays:  []int{1, 2, 3, 4, 5},
		WorkStart: "09:00",
		WorkEnd:   "11:00",
	}
	ss := NewSlotScheduler(user)

	// Saturday 2025-01-04 — only Monday 2025-01-06 should get slots within 3-day horizon
	slots := ss.BuildDailySlots(time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC))
	for _, slot := range slots {
		wd := int(slot.Date.Weekday())
		if wd == 0 {
			wd = 7
		}
		if wd == 6 || wd == 7 {
			t.Errorf("unexpected weekend slot on %v", slot.Date)
		}
	}
	if len(slots) == 0 {
		t.Error("expected slots on the next work day (Monday)")
	}
}
