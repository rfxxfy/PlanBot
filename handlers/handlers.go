package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/models"
	"github.com/adkhorst/planbot/scheduler"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot context for handlers
type BotHandler struct {
	bot *tgbotapi.BotAPI
}

// NewBotHandler creates a new bot handler
func NewBotHandler(bot *tgbotapi.BotAPI) *BotHandler {
	return &BotHandler{bot: bot}
}

// HandleUpdate processes incoming updates
func (h *BotHandler) HandleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	log.Printf("[%s] %s", msg.From.UserName, msg.Text)

	// Check if it's a command
	if msg.IsCommand() {
		h.handleCommand(msg)
		return
	}

	// Regular message handling if needed
	h.sendMessage(msg.Chat.ID, "Используйте /help для списка команд")
}

// handleCommand routes commands to appropriate handlers
func (h *BotHandler) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.handleStart(msg)
	case "help":
		h.handleHelp(msg)
	case "addtask":
		h.handleAddTask(msg)
	case "mytasks":
		h.handleMyTasks(msg)
	case "schedule":
		h.handleSchedule(msg)
	case "today":
		h.handleToday(msg)
	case "week":
		h.handleWeek(msg)
	case "complete":
		h.handleComplete(msg)
	case "delete":
		h.handleDelete(msg)
	case "settings":
		h.handleSettings(msg)
	default:
		h.sendMessage(msg.Chat.ID, "Неизвестная команда. Используйте /help")
	}
}

// handleStart handles /start command
func (h *BotHandler) handleStart(msg *tgbotapi.Message) {
	user, err := database.GetOrCreateUser(
		msg.From.ID,
		msg.From.UserName,
		msg.From.FirstName,
		msg.From.LastName,
	)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		h.sendMessage(msg.Chat.ID, "Произошла ошибка при регистрации")
		return
	}

	welcomeMsg := fmt.Sprintf(`Привет, %s! 👋

Я - PlanBot, твой помощник в планировании задач.

Я помогу тебе распределить задачи по дням с учётом:
• Времени, необходимого на каждую задачу
• Приоритетов
• Дедлайнов
• Твоей дневной нагрузки

Используй /help чтобы увидеть все команды.`, user.FirstName)

	h.sendMessage(msg.Chat.ID, welcomeMsg)
}

// handleHelp handles /help command
func (h *BotHandler) handleHelp(msg *tgbotapi.Message) {
	helpText := `📋 Доступные команды:

/addtask - Добавить новую задачу
Формат: /addtask Название | часы | приоритет | дедлайн
Пример: /addtask Написать отчёт | 4 | 5 | 25.12.2025

/mytasks - Список всех задач
/schedule - Распланировать задачи
/today - Расписание на сегодня
/week - Расписание на неделю
/complete <ID> - Отметить задачу выполненной
/delete <ID> - Удалить задачу
/settings - Настройки (часы в день, рабочие дни)

💡 Советы:
• Приоритет: 1-10 (10 = самый важный)
• Дедлайн необязателен
• Задачи автоматически распределяются по дням`

	h.sendMessage(msg.Chat.ID, helpText)
}

// handleAddTask handles /addtask command
func (h *BotHandler) handleAddTask(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя. Используйте /start")
		return
	}

	// Parse arguments: title | hours | priority | deadline
	args := msg.CommandArguments()
	if args == "" {
		h.sendMessage(msg.Chat.ID, "Формат: /addtask Название | часы | приоритет | дедлайн\nПример: /addtask Написать отчёт | 4 | 5 | 25.12.2025")
		return
	}

	parts := strings.Split(args, "|")
	if len(parts) < 2 {
		h.sendMessage(msg.Chat.ID, "Минимум укажите название и часы\nПример: /addtask Задача | 2")
		return
	}

	title := strings.TrimSpace(parts[0])
	hoursStr := strings.TrimSpace(parts[1])

	hours, err := strconv.ParseFloat(hoursStr, 64)
	if err != nil || hours <= 0 {
		h.sendMessage(msg.Chat.ID, "Неверное количество часов")
		return
	}

	task := &models.Task{
		UserID:        user.ID,
		Title:         title,
		HoursRequired: hours,
		Priority:      5, // default priority
	}

	// Parse priority if provided
	if len(parts) > 2 {
		priorityStr := strings.TrimSpace(parts[2])
		priority, err := strconv.Atoi(priorityStr)
		if err == nil && priority >= 1 && priority <= 10 {
			task.Priority = priority
		}
	}

	// Parse deadline if provided
	if len(parts) > 3 {
		deadlineStr := strings.TrimSpace(parts[3])
		deadline, err := parseDate(deadlineStr)
		if err == nil {
			task.Deadline = &deadline
		}
	}

	// Save task
	err = database.CreateTask(task)
	if err != nil {
		log.Printf("Error creating task: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка при создании задачи")
		return
	}

	response := fmt.Sprintf("✅ Задача создана!\n\n📝 %s\n⏱ %g часов\n⭐️ Приоритет: %d",
		task.Title, task.HoursRequired, task.Priority)

	if task.Deadline != nil {
		response += fmt.Sprintf("\n📅 Дедлайн: %s", task.Deadline.Format("02.01.2006"))
	}

	response += "\n\nИспользуйте /schedule для планирования"

	h.sendMessage(msg.Chat.ID, response)
}

// handleMyTasks handles /mytasks command
func (h *BotHandler) handleMyTasks(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	tasks, err := database.GetUserTasks(user.ID)
	if err != nil {
		log.Printf("Error getting tasks: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка получения задач")
		return
	}

	if len(tasks) == 0 {
		h.sendMessage(msg.Chat.ID, "У вас пока нет задач. Используйте /addtask")
		return
	}

	response := "📋 Ваши задачи:\n\n"
	for _, task := range tasks {
		statusEmoji := getStatusEmoji(task.Status)
		response += fmt.Sprintf("%s ID:%d | %s\n⏱ %g ч | ⭐️ %d",
			statusEmoji, task.ID, task.Title, task.HoursRequired, task.Priority)

		if task.Deadline != nil {
			response += fmt.Sprintf(" | 📅 %s", task.Deadline.Format("02.01.2006"))
		}
		response += "\n\n"
	}

	h.sendMessage(msg.Chat.ID, response)
}

// handleSchedule handles /schedule command
func (h *BotHandler) handleSchedule(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	// Get pending tasks
	tasks, err := database.GetPendingTasks(user.ID)
	if err != nil {
		log.Printf("Error getting pending tasks: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка получения задач")
		return
	}

	if len(tasks) == 0 {
		h.sendMessage(msg.Chat.ID, "Нет задач для планирования")
		return
	}

	h.sendMessage(msg.Chat.ID, "🔄 Планирую задачи...")

	// Run scheduler
	s := scheduler.NewScheduler(user, tasks)
	result := s.Schedule(time.Now())

	// Clear old schedules for these tasks
	taskIDs := make([]int64, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}
	database.ClearTaskSchedules(taskIDs)

	// Save new schedules
	if len(result.DaySchedules) > 0 {
		err = database.SaveTaskSchedules(result.DaySchedules)
		if err != nil {
			log.Printf("Error saving schedules: %v", err)
			h.sendMessage(msg.Chat.ID, "Ошибка сохранения расписания")
			return
		}
	}

	// Format response
	response := fmt.Sprintf("✅ %s\n\n", result.Message)

	if len(result.DaySchedules) > 0 {
		response += "📅 Расписание:\n\n"
		for i, daySchedule := range result.DaySchedules {
			if i >= 7 { // Show only first week
				response += fmt.Sprintf("\n... и ещё %d дней", len(result.DaySchedules)-7)
				break
			}
			response += formatDaySchedule(daySchedule, user.DailyCapacity)
		}
	}

	if len(result.UnscheduledTasks) > 0 {
		response += fmt.Sprintf("\n\n⚠️ Не удалось запланировать %d задач(и)", len(result.UnscheduledTasks))
	}

	h.sendMessage(msg.Chat.ID, response)
}

// handleToday handles /today command
func (h *BotHandler) handleToday(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	today := time.Now()
	schedules, err := database.GetScheduleForDateRange(user.ID, today, today)
	if err != nil {
		log.Printf("Error getting schedule: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка получения расписания")
		return
	}

	if len(schedules) == 0 {
		h.sendMessage(msg.Chat.ID, "📭 На сегодня нет запланированных задач")
		return
	}

	response := "📅 Сегодня:\n\n"
	response += formatDaySchedule(schedules[0], user.DailyCapacity)

	h.sendMessage(msg.Chat.ID, response)
}

// handleWeek handles /week command
func (h *BotHandler) handleWeek(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	today := time.Now()
	endDate := today.AddDate(0, 0, 7)

	schedules, err := database.GetScheduleForDateRange(user.ID, today, endDate)
	if err != nil {
		log.Printf("Error getting schedule: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка получения расписания")
		return
	}

	if len(schedules) == 0 {
		h.sendMessage(msg.Chat.ID, "📭 На эту неделю нет запланированных задач")
		return
	}

	response := "📅 Расписание на неделю:\n\n"
	for _, daySchedule := range schedules {
		response += formatDaySchedule(daySchedule, user.DailyCapacity)
	}

	h.sendMessage(msg.Chat.ID, response)
}

// handleComplete handles /complete command
func (h *BotHandler) handleComplete(msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		h.sendMessage(msg.Chat.ID, "Укажите ID задачи: /complete <ID>")
		return
	}

	taskID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Неверный ID задачи")
		return
	}

	err = database.CompleteTask(taskID)
	if err != nil {
		log.Printf("Error completing task: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка при отметке задачи")
		return
	}

	h.sendMessage(msg.Chat.ID, "✅ Задача отмечена как выполненная!")
}

// handleDelete handles /delete command
func (h *BotHandler) handleDelete(msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		h.sendMessage(msg.Chat.ID, "Укажите ID задачи: /delete <ID>")
		return
	}

	taskID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Неверный ID задачи")
		return
	}

	err = database.DeleteTask(taskID)
	if err != nil {
		log.Printf("Error deleting task: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка при удалении задачи")
		return
	}

	h.sendMessage(msg.Chat.ID, "🗑 Задача удалена")
}

// handleSettings handles /settings command
func (h *BotHandler) handleSettings(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	args := msg.CommandArguments()
	if args == "" {
		// Show current settings
		workDaysStr := formatWorkDays(user.WorkDays)
		response := fmt.Sprintf(`⚙️ Текущие настройки:

⏰ Часов в день: %.1f
📅 Рабочие дни: %s

Для изменения используйте:
/settings <часы> | <дни>
Пример: /settings 6 | 1,2,3,4,5`, user.DailyCapacity, workDaysStr)

		h.sendMessage(msg.Chat.ID, response)
		return
	}

	// Parse new settings
	parts := strings.Split(args, "|")
	if len(parts) != 2 {
		h.sendMessage(msg.Chat.ID, "Формат: /settings <часы> | <дни>\nПример: /settings 6 | 1,2,3,4,5")
		return
	}

	hoursStr := strings.TrimSpace(parts[0])
	hours, err := strconv.ParseFloat(hoursStr, 64)
	if err != nil || hours <= 0 || hours > 24 {
		h.sendMessage(msg.Chat.ID, "Неверное количество часов (должно быть от 0 до 24)")
		return
	}

	daysStr := strings.TrimSpace(parts[1])
	daysParts := strings.Split(daysStr, ",")
	workDays := []int{}
	for _, dayStr := range daysParts {
		day, err := strconv.Atoi(strings.TrimSpace(dayStr))
		if err != nil || day < 1 || day > 7 {
			h.sendMessage(msg.Chat.ID, "Неверный день недели (1=Пн, 7=Вс)")
			return
		}
		workDays = append(workDays, day)
	}

	err = database.UpdateUserSettings(user.ID, hours, workDays)
	if err != nil {
		log.Printf("Error updating settings: %v", err)
		h.sendMessage(msg.Chat.ID, "Ошибка при обновлении настроек")
		return
	}

	h.sendMessage(msg.Chat.ID, "✅ Настройки обновлены!")
}

// Helper functions

func (h *BotHandler) getUser(telegramID int64) (*models.User, error) {
	return database.GetOrCreateUser(telegramID, "", "", "")
}

func (h *BotHandler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"

	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func getStatusEmoji(status string) string {
	switch status {
	case "pending":
		return "⏳"
	case "scheduled":
		return "📅"
	case "in_progress":
		return "🔄"
	case "completed":
		return "✅"
	case "cancelled":
		return "❌"
	default:
		return "❓"
	}
}

func formatDaySchedule(daySchedule models.DaySchedule, dailyCapacity float64) string {
	weekday := getWeekdayRu(daySchedule.Date.Weekday())
	result := fmt.Sprintf("📆 %s, %s\n", weekday, daySchedule.Date.Format("02.01.2006"))
	result += fmt.Sprintf("⏱ Нагрузка: %.1f / %.1f ч\n\n", daySchedule.TotalHours, dailyCapacity)

	for _, task := range daySchedule.Tasks {
		result += fmt.Sprintf("• %s (%.1f ч) ⭐️ %d\n", task.Title, task.HoursAllocated, task.Priority)
	}
	result += "\n"

	return result
}

func formatWorkDays(workDays []int) string {
	days := []string{}
	dayNames := map[int]string{
		1: "Пн", 2: "Вт", 3: "Ср", 4: "Чт", 5: "Пт", 6: "Сб", 7: "Вс",
	}
	for _, day := range workDays {
		days = append(days, dayNames[day])
	}
	return strings.Join(days, ", ")
}

func getWeekdayRu(weekday time.Weekday) string {
	days := map[time.Weekday]string{
		time.Monday:    "Понедельник",
		time.Tuesday:   "Вторник",
		time.Wednesday: "Среда",
		time.Thursday:  "Четверг",
		time.Friday:    "Пятница",
		time.Saturday:  "Суббота",
		time.Sunday:    "Воскресенье",
	}
	return days[weekday]
}

func parseDate(dateStr string) (time.Time, error) {
	// Try DD.MM.YYYY format
	t, err := time.Parse("02.01.2006", dateStr)
	if err == nil {
		return t, nil
	}

	// Try DD.MM.YY format
	t, err = time.Parse("02.01.06", dateStr)
	if err == nil {
		return t, nil
	}

	// Try YYYY-MM-DD format
	t, err = time.Parse("2006-01-02", dateStr)
	return t, err
}
