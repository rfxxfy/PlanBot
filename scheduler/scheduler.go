package scheduler

import (
	"math"
	"sort"
	"time"

	"github.com/adkhorst/planbot/models"
)

// Scheduler handles task scheduling logic
type Scheduler struct {
	user  *models.User
	tasks []models.Task
}

// NewScheduler creates a new scheduler instance
func NewScheduler(user *models.User, tasks []models.Task) *Scheduler {
	return &Scheduler{
		user:  user,
		tasks: tasks,
	}
}

// Schedule distributes tasks across days using deadline-aware algorithm
func (s *Scheduler) Schedule(startDate time.Time) *models.ScheduleResult {
	result := &models.ScheduleResult{
		Success:          true,
		DaySchedules:     []models.DaySchedule{},
		UnscheduledTasks: []int64{},
	}

	// Filter only pending tasks
	pendingTasks := s.filterPendingTasks()
	if len(pendingTasks) == 0 {
		result.Message = "Нет задач для планирования"
		return result
	}

	// Sort tasks by priority and deadline
	sortedTasks := s.sortTasksByDeadlineAndPriority(pendingTasks)

	// Create day slots map
	daySlots := make(map[string]*models.DaySchedule)

	// Schedule tasks
	for _, task := range sortedTasks {
		scheduled := s.scheduleTask(task, startDate, daySlots)
		if !scheduled {
			result.UnscheduledTasks = append(result.UnscheduledTasks, task.ID)
			result.Success = false
		}
	}

	// Convert map to sorted slice
	result.DaySchedules = s.convertDaySlotsToSlice(daySlots)

	if len(result.UnscheduledTasks) > 0 {
		result.Message = "Некоторые задачи не удалось запланировать"
	} else {
		result.Message = "Все задачи успешно запланированы"
	}

	return result
}

// filterPendingTasks returns only tasks with pending status
func (s *Scheduler) filterPendingTasks() []models.Task {
	pending := []models.Task{}
	for _, task := range s.tasks {
		if task.Status == "pending" {
			pending = append(pending, task)
		}
	}
	return pending
}

// sortTasksByDeadlineAndPriority sorts tasks by deadline (closest first) and priority
func (s *Scheduler) sortTasksByDeadlineAndPriority(tasks []models.Task) []models.Task {
	sorted := make([]models.Task, len(tasks))
	copy(sorted, tasks)

	sort.Slice(sorted, func(i, j int) bool {
		// Tasks with deadlines come first
		if sorted[i].Deadline != nil && sorted[j].Deadline == nil {
			return true
		}
		if sorted[i].Deadline == nil && sorted[j].Deadline != nil {
			return false
		}

		// If both have deadlines, sort by deadline
		if sorted[i].Deadline != nil && sorted[j].Deadline != nil {
			if !sorted[i].Deadline.Equal(*sorted[j].Deadline) {
				return sorted[i].Deadline.Before(*sorted[j].Deadline)
			}
		}

		// If deadlines are equal or both don't have deadlines, sort by priority
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}

		// If priority is equal, sort by hours (smaller tasks first)
		return sorted[i].HoursRequired < sorted[j].HoursRequired
	})

	return sorted
}

// scheduleTask attempts to schedule a single task
func (s *Scheduler) scheduleTask(task models.Task, startDate time.Time, daySlots map[string]*models.DaySchedule) bool {
	remainingHours := task.HoursRequired
	currentDate := s.normalizeDate(startDate)

	// Calculate latest possible start date if there's a deadline
	var latestStartDate time.Time
	if task.Deadline != nil {
		// Work backwards from deadline
		latestStartDate = s.calculateLatestStartDate(*task.Deadline, task.HoursRequired)
		if latestStartDate.Before(currentDate) {
			// Task can't be completed before deadline
			return false
		}
	}

	// Try to allocate hours across available days
	maxDaysToCheck := 365 // Don't look more than a year ahead
	daysChecked := 0

	for remainingHours > 0 && daysChecked < maxDaysToCheck {
		// Check if this day is a work day
		if !s.isWorkDay(currentDate) {
			currentDate = currentDate.AddDate(0, 0, 1)
			daysChecked++
			continue
		}

		// Check if we've exceeded the deadline
		if task.Deadline != nil && currentDate.After(*task.Deadline) {
			return false
		}

		// Get or create day slot
		dateKey := s.formatDate(currentDate)
		daySlot, exists := daySlots[dateKey]
		if !exists {
			daySlot = &models.DaySchedule{
				Date:           currentDate,
				Tasks:          []models.ScheduledTaskInfo{},
				TotalHours:     0,
				AvailableHours: s.user.DailyCapacity,
			}
			daySlots[dateKey] = daySlot
		}

		// Calculate available hours for this day
		availableHours := s.user.DailyCapacity - daySlot.TotalHours

		if availableHours > 0 {
			// Allocate as much as possible to this day
			hoursToAllocate := remainingHours
			if hoursToAllocate > availableHours {
				hoursToAllocate = availableHours
			}

			// Add task to this day
			daySlot.Tasks = append(daySlot.Tasks, models.ScheduledTaskInfo{
				TaskID:         task.ID,
				Title:          task.Title,
				HoursAllocated: hoursToAllocate,
				Priority:       task.Priority,
				Deadline:       task.Deadline,
			})
			daySlot.TotalHours += hoursToAllocate
			daySlot.AvailableHours = s.user.DailyCapacity - daySlot.TotalHours

			remainingHours -= hoursToAllocate
		}

		currentDate = currentDate.AddDate(0, 0, 1)
		daysChecked++
	}

	return remainingHours == 0
}

// calculateLatestStartDate calculates the latest date a task can start
func (s *Scheduler) calculateLatestStartDate(deadline time.Time, hoursRequired float64) time.Time {
	// Правильное вычисление необходимого количества рабочих дней
	// Используем math.Ceil для округления вверх
	workDaysNeeded := int(math.Ceil(hoursRequired / s.user.DailyCapacity))
	if workDaysNeeded < 1 {
		workDaysNeeded = 1 // Минимум один день
	}

	date := deadline
	workDaysFound := 0

	// Идем назад от дедлайна, считая только рабочие дни
	for workDaysFound < workDaysNeeded {
		date = date.AddDate(0, 0, -1)
		if s.isWorkDay(date) {
			workDaysFound++
		}
	}

	return s.normalizeDate(date)
}

// isWorkDay checks if a date is a work day for the user
func (s *Scheduler) isWorkDay(date time.Time) bool {
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}

	for _, workDay := range s.user.WorkDays {
		if workDay == weekday {
			return true
		}
	}
	return false
}

// normalizeDate removes time component from date
func (s *Scheduler) normalizeDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// formatDate formats date as YYYY-MM-DD
func (s *Scheduler) formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// convertDaySlotsToSlice converts map to sorted slice
func (s *Scheduler) convertDaySlotsToSlice(daySlots map[string]*models.DaySchedule) []models.DaySchedule {
	schedules := make([]models.DaySchedule, 0, len(daySlots))
	for _, slot := range daySlots {
		schedules = append(schedules, *slot)
	}

	// Sort by date
	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].Date.Before(schedules[j].Date)
	})

	return schedules
}
