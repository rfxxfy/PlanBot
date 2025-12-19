package models

import (
	"time"
)

// User represents a Telegram user
type User struct {
	ID            int64
	TelegramID    int64
	Username      string
	FirstName     string
	LastName      string
	DailyCapacity float64 // hours per day
	WorkDays      []int   // 1=Monday, 7=Sunday
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Task represents a user's task
type Task struct {
	ID            int64
	UserID        int64
	Title         string
	Description   string
	HoursRequired float64
	Priority      int
	Status        string // pending, scheduled, in_progress, completed, cancelled
	Deadline      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time
}

// TaskSchedule represents when a task is scheduled
type TaskSchedule struct {
	ID             int64
	TaskID         int64
	ScheduledDate  time.Time
	HoursAllocated float64
	CreatedAt      time.Time
}

// DaySchedule represents all tasks scheduled for a specific day
type DaySchedule struct {
	Date           time.Time
	Tasks          []ScheduledTaskInfo
	TotalHours     float64
	AvailableHours float64
}

// ScheduledTaskInfo contains task details with scheduling info
type ScheduledTaskInfo struct {
	TaskID         int64
	Title          string
	HoursAllocated float64
	Priority       int
	Deadline       *time.Time
}

// ScheduleRequest represents a request to schedule tasks
type ScheduleRequest struct {
	UserID    int64
	StartDate time.Time
}

// ScheduleResult represents the result of scheduling
type ScheduleResult struct {
	Success          bool
	Message          string
	DaySchedules     []DaySchedule
	UnscheduledTasks []int64 // IDs of tasks that couldn't be scheduled
}
