package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
)

func TestParseDate_Formats(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"25.12.2025", time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)},
		{"25.12.25", time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)},
		{"2025-12-25", time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDate(tc.input)
			if err != nil {
				t.Fatalf("parseDate(%q) error: %v", tc.input, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("parseDate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDate_Invalid(t *testing.T) {
	if _, err := parseDate("not-a-date"); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestParseCallbackTaskID(t *testing.T) {
	id, err := parseCallbackTaskID("plan_insert:42", "plan_insert:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected id 42, got %d", id)
	}

	_, err = parseCallbackTaskID("plan_insert:abc", "plan_insert:")
	if err == nil {
		t.Error("expected error for non-numeric id")
	}
}

func TestFormatWorkDays(t *testing.T) {
	got := formatWorkDays([]int{1, 3, 5})
	want := "Пн, Ср, Пт"
	if got != want {
		t.Errorf("formatWorkDays() = %q, want %q", got, want)
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := map[string]string{
		"pending":     "⏳",
		"scheduled":   "📅",
		"in_progress": "🔄",
		"completed":   "✅",
		"cancelled":   "❌",
		"unknown":     "❓",
	}
	for status, want := range tests {
		if got := getStatusEmoji(status); got != want {
			t.Errorf("getStatusEmoji(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestGetWeekdayRu(t *testing.T) {
	if got := getWeekdayRu(time.Monday); got != "Понедельник" {
		t.Errorf("unexpected weekday name: %q", got)
	}
}

func TestFilterAllocationsByDate(t *testing.T) {
	loc := time.UTC
	allocations := []models.SlotAllocation{
		{Start: time.Date(2025, 1, 6, 9, 0, 0, 0, loc), End: time.Date(2025, 1, 6, 10, 0, 0, 0, loc)},
		{Start: time.Date(2025, 1, 7, 9, 0, 0, 0, loc), End: time.Date(2025, 1, 7, 10, 0, 0, 0, loc)},
	}

	filtered := filterAllocationsByDate(allocations, "2025-01-06")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(filtered))
	}
	if filtered[0].Start.Day() != 6 {
		t.Errorf("unexpected filtered allocation day: %d", filtered[0].Start.Day())
	}
}

func TestFormatDaySchedule(t *testing.T) {
	day := models.DaySchedule{
		Date:       time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
		TotalHours: 3,
		Tasks: []models.ScheduledTaskInfo{
			{Title: "Write tests", HoursAllocated: 3, Priority: 8},
		},
	}

	out := formatDaySchedule(day, 8)
	if out == "" {
		t.Fatal("expected non-empty schedule text")
	}
	for _, part := range []string{"Понедельник", "Write tests", "3.0", "⭐️ 8"} {
		if !strings.Contains(out, part) {
			t.Errorf("expected output to contain %q, got: %q", part, out)
		}
	}
}
