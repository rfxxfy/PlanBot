package scheduler

import (
	"sort"
	"time"

	"github.com/adkhorst/planbot/models"
)

// ScheduleTaskIntoExisting places one new task into free slots, keeping existing day plans unchanged.
func ScheduleTaskIntoExisting(user *models.User, newTask models.Task, existing []models.DaySchedule, startDate time.Time, busy []models.BusyInterval) ([]models.DaySchedule, bool) {
	if newTask.HoursRequired <= 0 {
		return nil, false
	}

	slotScheduler := NewSlotScheduler(user)
	slots := BuildWorkSlots(user, startDate, busy)

	// Occupy slots with already planned tasks (same slot grid).
	_ = applyDaySchedulesToSlots(slots, existing)

	slotsByDate := indexSlotsByDate(slots)
	daySlots := make(map[string]*models.DaySchedule)
	remaining := newTask.HoursRequired

	horizon := slotScheduler.horizonDays
	current := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	daysChecked := 0

	for remaining > 0 && daysChecked < horizon {
		if !slotScheduler.isWorkDay(current) {
			current = current.AddDate(0, 0, 1)
			daysChecked++
			continue
		}

		if newTask.Deadline != nil {
			deadlineDay := time.Date(newTask.Deadline.Year(), newTask.Deadline.Month(), newTask.Deadline.Day(), 0, 0, 0, 0, current.Location())
			if current.After(deadlineDay) {
				return convertDayMapToSlice(daySlots), false
			}
		}

		dateKey := current.Format("2006-01-02")
		daySlotList := slotsByDate[dateKey]

		for _, slot := range daySlotList {
			if remaining <= 0 {
				break
			}
			free := slot.CapacityHours - slot.AllocatedHours
			if free <= 1e-9 {
				continue
			}

			toAllocate := remaining
			if toAllocate > free {
				toAllocate = free
			}

			ds, ok := daySlots[dateKey]
			if !ok {
				ds = &models.DaySchedule{
					Date:           current,
					Tasks:          []models.ScheduledTaskInfo{},
					TotalHours:     0,
					AvailableHours: user.DailyCapacity,
				}
				daySlots[dateKey] = ds
			}

			ds.Tasks = append(ds.Tasks, models.ScheduledTaskInfo{
				TaskID:         newTask.ID,
				Title:          newTask.Title,
				HoursAllocated: toAllocate,
				Priority:       newTask.Priority,
				Deadline:       newTask.Deadline,
			})
			ds.TotalHours += toAllocate
			slot.AllocatedHours += toAllocate
			remaining -= toAllocate
		}

		current = current.AddDate(0, 0, 1)
		daysChecked++
	}

	if remaining > 1e-9 {
		return convertDayMapToSlice(daySlots), false
	}
	return convertDayMapToSlice(daySlots), true
}

func indexSlotsByDate(slots []models.TimeSlot) map[string][]*models.TimeSlot {
	m := make(map[string][]*models.TimeSlot)
	for i := range slots {
		key := slots[i].Date.Format("2006-01-02")
		m[key] = append(m[key], &slots[i])
	}
	return m
}

func convertDayMapToSlice(daySlots map[string]*models.DaySchedule) []models.DaySchedule {
	if len(daySlots) == 0 {
		return nil
	}
	out := make([]models.DaySchedule, 0, len(daySlots))
	for _, ds := range daySlots {
		out = append(out, *ds)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Date.Before(out[j].Date)
	})
	return out
}
