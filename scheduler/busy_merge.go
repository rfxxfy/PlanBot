package scheduler

import (
	"sort"
	"time"

	"github.com/adkhorst/planbot/models"
)

// MergeBusyIntervals combines and sorts busy intervals (later passes win on overlap in BlockSlotsFromBusy).
func MergeBusyIntervals(parts ...[]models.BusyInterval) []models.BusyInterval {
	var all []models.BusyInterval
	for _, p := range parts {
		all = append(all, p...)
	}
	if len(all) == 0 {
		return nil
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Start.Before(all[j].Start)
	})
	return all
}

// BusyHoursOnDate returns total busy hours on a date inside working slots (approximate).
func BusyHoursOnDate(busy []models.BusyInterval, date time.Time) float64 {
	key := date.Format("2006-01-02")
	var total float64
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	for _, b := range busy {
		if b.Start.Format("2006-01-02") != key && b.End.Format("2006-01-02") != key {
			// include cross-midnight overlap with this day
			if !b.Start.Before(dayEnd) || !b.End.After(dayStart) {
				continue
			}
		}
		total += overlapHours(dayStart, dayEnd, b.Start, b.End)
	}
	return total
}
