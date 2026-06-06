package scheduler

import (
	"sort"
	"time"

	"github.com/adkhorst/planbot/models"
)

// PlanTimeAllocations maps a day-level plan onto concrete time slots within working hours.
// Calendar busy is applied first so times do not overlap existing calendar events.
func PlanTimeAllocations(user *models.User, daySchedules []models.DaySchedule, startDate time.Time, busy []models.BusyInterval) []models.SlotAllocation {
	if len(daySchedules) == 0 {
		return nil
	}

	slots := BuildWorkSlots(user, startDate, busy)
	return applyDaySchedulesToSlots(slots, daySchedules)
}

// applyDaySchedulesToSlots fills slots from day-level plans and returns merged timed allocations.
func applyDaySchedulesToSlots(slots []models.TimeSlot, daySchedules []models.DaySchedule) []models.SlotAllocation {
	slotsByDate := indexSlotsByDate(slots)
	var allocations []models.SlotAllocation

	for _, day := range daySchedules {
		dateKey := day.Date.Format("2006-01-02")
		daySlots := slotsByDate[dateKey]
		if len(daySlots) == 0 {
			continue
		}

		slotIdx := 0
		for _, task := range day.Tasks {
			remaining := task.HoursAllocated
			for remaining > 0 && slotIdx < len(daySlots) {
				slot := daySlots[slotIdx]
				free := slot.CapacityHours - slot.AllocatedHours
				if free <= 1e-9 {
					slotIdx++
					continue
				}

				toAllocate := remaining
				if toAllocate > free {
					toAllocate = free
				}

				allocStart := slot.Start.Add(time.Duration(slot.AllocatedHours * float64(time.Hour)))
				allocEnd := allocStart.Add(time.Duration(toAllocate * float64(time.Hour)))

				allocations = append(allocations, models.SlotAllocation{
					TaskID:   task.TaskID,
					Title:    task.Title,
					Priority: task.Priority,
					Deadline: task.Deadline,
					Start:    allocStart,
					End:      allocEnd,
				})

				slot.AllocatedHours += toAllocate
				remaining -= toAllocate

				if slot.AllocatedHours >= slot.CapacityHours-1e-9 {
					slotIdx++
				}
			}
		}
	}

	return MergeSlotAllocations(allocations)
}

// MergeSlotAllocations joins consecutive blocks of the same task into one interval.
func MergeSlotAllocations(allocations []models.SlotAllocation) []models.SlotAllocation {
	if len(allocations) == 0 {
		return nil
	}

	sort.Slice(allocations, func(i, j int) bool {
		if allocations[i].Start.Equal(allocations[j].Start) {
			return allocations[i].TaskID < allocations[j].TaskID
		}
		return allocations[i].Start.Before(allocations[j].Start)
	})

	merged := []models.SlotAllocation{allocations[0]}
	for i := 1; i < len(allocations); i++ {
		cur := allocations[i]
		last := &merged[len(merged)-1]
		if cur.TaskID == last.TaskID && cur.Start.Equal(last.End) {
			last.End = cur.End
			continue
		}
		merged = append(merged, cur)
	}

	return merged
}
