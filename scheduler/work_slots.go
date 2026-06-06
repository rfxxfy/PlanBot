package scheduler

import (
	"os"
	"strconv"
	"time"

	"github.com/adkhorst/planbot/models"
)

// PlanningHorizonDays returns configured planning horizon.
func PlanningHorizonDays() int {
	horizon := 365
	if env := os.Getenv("PLANNING_HORIZON_DAYS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			horizon = v
		}
	}
	return horizon
}

// BuildWorkSlots creates the slot grid and marks Google Calendar busy times as occupied.
func BuildWorkSlots(user *models.User, startDate time.Time, busy []models.BusyInterval) []models.TimeSlot {
	slotScheduler := NewSlotScheduler(user)
	slots := slotScheduler.BuildDailySlots(startDate)
	BlockSlotsFromBusy(slots, busy)
	return slots
}

// BlockSlotsFromBusy marks overlapping parts of work slots as unavailable.
func BlockSlotsFromBusy(slots []models.TimeSlot, busy []models.BusyInterval) {
	for i := range slots {
		for _, b := range busy {
			overlapH := overlapHours(slots[i].Start, slots[i].End, b.Start, b.End)
			if overlapH <= 1e-9 {
				continue
			}
			slots[i].AllocatedHours += overlapH
			if slots[i].AllocatedHours > slots[i].CapacityHours {
				slots[i].AllocatedHours = slots[i].CapacityHours
			}
			if slots[i].Source == "" {
				slots[i].Source = "calendar"
			}
		}
	}
}

// FreeHoursOnDate returns remaining bookable hours on a date from the slot grid.
func FreeHoursOnDate(slots []models.TimeSlot, dateKey string) float64 {
	var free float64
	for i := range slots {
		if slots[i].Date.Format("2006-01-02") != dateKey {
			continue
		}
		rem := slots[i].CapacityHours - slots[i].AllocatedHours
		if rem > 0 {
			free += rem
		}
	}
	return free
}

func overlapHours(aStart, aEnd, bStart, bEnd time.Time) float64 {
	latest := aStart
	if bStart.After(latest) {
		latest = bStart
	}
	earliest := aEnd
	if bEnd.Before(earliest) {
		earliest = bEnd
	}
	if !earliest.After(latest) {
		return 0
	}
	return earliest.Sub(latest).Hours()
}

// HorizonEndDate returns the last day included in the planning horizon.
func HorizonEndDate(startDate time.Time) time.Time {
	return startDate.AddDate(0, 0, PlanningHorizonDays())
}

// allocateOnSlots marks hours as used on the slot grid; returns hours actually placed.
func allocateOnSlots(slots []models.TimeSlot, dateKey string, hours float64) float64 {
	remaining := hours
	var placed float64
	for i := range slots {
		if slots[i].Date.Format("2006-01-02") != dateKey {
			continue
		}
		free := slots[i].CapacityHours - slots[i].AllocatedHours
		if free <= 1e-9 {
			continue
		}
		toAllocate := remaining
		if toAllocate > free {
			toAllocate = free
		}
		slots[i].AllocatedHours += toAllocate
		remaining -= toAllocate
		placed += toAllocate
		if remaining <= 1e-9 {
			break
		}
	}
	return placed
}
