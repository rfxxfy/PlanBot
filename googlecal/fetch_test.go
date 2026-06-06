package googlecal

import (
	"testing"
	"time"

	"github.com/adkhorst/planbot/models"
	"google.golang.org/api/calendar/v3"
)

func TestIsPlanBotCalendarEvent(t *testing.T) {
	tests := []struct {
		name string
		ev   *calendar.Event
		want bool
	}{
		{
			name: "extended property",
			ev: &calendar.Event{
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{"planbot": "1"},
				},
			},
			want: true,
		},
		{
			name: "description prefix",
			ev:   &calendar.Event{Description: "PlanBot\nsomething"},
			want: true,
		},
		{
			name: "external event",
			ev:   &calendar.Event{Summary: "Meeting", Description: "Team sync"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPlanBotCalendarEvent(tc.ev); got != tc.want {
				t.Errorf("isPlanBotCalendarEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseEventDateTime_RFC3339(t *testing.T) {
	loc := time.UTC
	dt := &calendar.EventDateTime{
		DateTime: "2025-06-02T14:30:00Z",
	}
	got, err := parseEventDateTime(dt, loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2025, 6, 2, 14, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseEventDateTime_AllDayDate(t *testing.T) {
	loc := time.UTC
	dt := &calendar.EventDateTime{Date: "2025-06-02"}
	got, err := parseEventDateTime(dt, loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEventToBusyInterval_TimedEvent(t *testing.T) {
	loc := time.UTC
	ev := &calendar.Event{
		Summary: "Standup",
		Start:   &calendar.EventDateTime{DateTime: "2025-06-02T10:00:00Z"},
		End:     &calendar.EventDateTime{DateTime: "2025-06-02T10:30:00Z"},
	}
	user := &models.User{WorkStart: "09:00", WorkEnd: "18:00"}

	interval, ok := eventToBusyInterval(ev, loc, user)
	if !ok {
		t.Fatal("expected valid busy interval")
	}
	if interval.Summary != "Standup" {
		t.Errorf("summary = %q", interval.Summary)
	}
	if interval.End.Sub(interval.Start).Minutes() != 30 {
		t.Errorf("expected 30 minutes, got %v", interval.End.Sub(interval.Start))
	}
}

func TestAllDayBusyInterval_UsesWorkHours(t *testing.T) {
	loc := time.UTC
	day := time.Date(2025, 6, 2, 0, 0, 0, 0, loc)
	user := &models.User{WorkStart: "10:00", WorkEnd: "14:00"}

	interval := allDayBusyInterval(user, day, "Holiday")
	if interval.Start.Hour() != 10 || interval.End.Hour() != 14 {
		t.Errorf("expected work hours 10-14, got %v-%v", interval.Start, interval.End)
	}
	if !interval.AllDay {
		t.Error("expected AllDay=true")
	}
}

func TestEventTimeRange_AllDay(t *testing.T) {
	loc := time.UTC
	ev := &calendar.Event{
		Start: &calendar.EventDateTime{Date: "2025-06-02"},
		End:   &calendar.EventDateTime{Date: "2025-06-03"},
	}
	start, end, allDay, ok := eventTimeRange(ev, loc)
	if !ok || !allDay {
		t.Fatalf("expected valid all-day range, ok=%v allDay=%v", ok, allDay)
	}
	if !start.Equal(time.Date(2025, 6, 2, 0, 0, 0, 0, loc)) {
		t.Errorf("unexpected start: %v", start)
	}
	if !end.Equal(time.Date(2025, 6, 3, 0, 0, 0, 0, loc)) {
		t.Errorf("unexpected end: %v", end)
	}
}

func TestUserLocation_Fallback(t *testing.T) {
	loc := userLocation(&models.User{TimeZone: "Invalid/Timezone"})
	if loc != time.UTC {
		t.Errorf("expected UTC fallback, got %v", loc)
	}
}
