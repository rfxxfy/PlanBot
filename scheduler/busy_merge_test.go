package scheduler

import (
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestMergeBusyIntervals_Empty(t *testing.T) {
	if got := MergeBusyIntervals(nil, []models.BusyInterval{}); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestMergeBusyIntervals_SortsByStart(t *testing.T) {
	loc := time.UTC
	a := time.Date(2025, 3, 10, 14, 0, 0, 0, loc)
	b := time.Date(2025, 3, 10, 10, 0, 0, 0, loc)

	merged := MergeBusyIntervals(
		[]models.BusyInterval{{Start: a, End: a.Add(time.Hour)}},
		[]models.BusyInterval{{Start: b, End: b.Add(time.Hour)}},
	)

	if len(merged) != 2 {
		t.Fatalf("expected 2 intervals, got %d", len(merged))
	}
	if !merged[0].Start.Equal(b) {
		t.Errorf("first interval should start at 10:00, got %v", merged[0].Start)
	}
}

func TestBusyHoursOnDate_SingleOverlap(t *testing.T) {
	loc := time.UTC
	day := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	busy := []models.BusyInterval{
		{
			Start: time.Date(2025, 6, 2, 10, 0, 0, 0, loc),
			End:   time.Date(2025, 6, 2, 12, 30, 0, 0, loc),
		},
	}

	hours := BusyHoursOnDate(busy, day)
	if hours != 2.5 {
		t.Errorf("expected 2.5 busy hours, got %f", hours)
	}
}

func TestBusyHoursOnDate_CrossMidnight(t *testing.T) {
	loc := time.UTC
	day := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	busy := []models.BusyInterval{
		{
			Start: time.Date(2025, 6, 1, 23, 0, 0, 0, loc),
			End:   time.Date(2025, 6, 2, 1, 0, 0, 0, loc),
		},
	}

	hours := BusyHoursOnDate(busy, day)
	if hours != 1.0 {
		t.Errorf("expected 1.0 hour crossing midnight, got %f", hours)
	}
}

func TestOverlapHours(t *testing.T) {
	loc := time.UTC
	tests := []struct {
		name string
		aS   time.Time
		aE   time.Time
		bS   time.Time
		bE   time.Time
		want float64
	}{
		{
			name: "full overlap",
			aS:   time.Date(2025, 1, 1, 9, 0, 0, 0, loc),
			aE:   time.Date(2025, 1, 1, 11, 0, 0, 0, loc),
			bS:   time.Date(2025, 1, 1, 9, 30, 0, 0, loc),
			bE:   time.Date(2025, 1, 1, 10, 30, 0, 0, loc),
			want: 1.0,
		},
		{
			name: "no overlap",
			aS:   time.Date(2025, 1, 1, 9, 0, 0, 0, loc),
			aE:   time.Date(2025, 1, 1, 10, 0, 0, 0, loc),
			bS:   time.Date(2025, 1, 1, 11, 0, 0, 0, loc),
			bE:   time.Date(2025, 1, 1, 12, 0, 0, 0, loc),
			want: 0,
		},
		{
			name: "touching edges",
			aS:   time.Date(2025, 1, 1, 9, 0, 0, 0, loc),
			aE:   time.Date(2025, 1, 1, 10, 0, 0, 0, loc),
			bS:   time.Date(2025, 1, 1, 10, 0, 0, 0, loc),
			bE:   time.Date(2025, 1, 1, 11, 0, 0, 0, loc),
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := overlapHours(tc.aS, tc.aE, tc.bS, tc.bE)
			if got != tc.want {
				t.Errorf("overlapHours() = %f, want %f", got, tc.want)
			}
		})
	}
}
