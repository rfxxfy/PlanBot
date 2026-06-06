package handlers

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/googlecal"
	"github.com/adkhorst/planbot/models"
	"github.com/adkhorst/planbot/scheduler"
)

type scheduleOutcome struct {
	modeLabel        string
	result           *models.ScheduleResult
	timeAllocations  []models.SlotAllocation
	scheduledCount   int
	totalTasks       int
	calendarSynced   bool
	calendarSyncFail bool
	syncErrorDetail  string
}

func (h *BotHandler) executeFullRebuild(chatID int64, user *models.User) {
	tasks, err := database.GetActiveTasks(user.ID)
	if err != nil {
		log.Printf("Error getting active tasks: %v", err)
		h.sendMessage(chatID, "Ошибка получения задач из базы.\nПопробуйте позже.")
		return
	}
	if len(tasks) == 0 {
		h.sendMessage(chatID, "Нет активных задач для планирования.\nДобавьте задачу через /addtask.")
		return
	}

	startDate := scheduleStartDate(user)
	h.clearPlanBotCalendar(user)
	busy := h.fetchCalendarBusy(user, startDate, true)
	workSlots := scheduler.BuildWorkSlots(user, startDate, busy)
	s := scheduler.NewSchedulerWithSlots(user, tasks, workSlots)
	result := s.Schedule(startDate)
	timeAllocations := scheduler.PlanTimeAllocations(user, result.DaySchedules, startDate, busy)

	taskIDs := make([]int64, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}
	_ = database.ClearTaskSchedules(taskIDs)

	if len(result.DaySchedules) > 0 {
		if err := database.SaveTaskSchedules(result.DaySchedules); err != nil {
			log.Printf("Error saving schedules: %v", err)
			h.sendMessage(chatID, "Ошибка сохранения расписания")
			return
		}
	}

	for _, unscheduledID := range result.UnscheduledTasks {
		_ = database.UpdateTaskStatus(unscheduledID, "pending")
	}

	outcome := scheduleOutcome{
		modeLabel:       "полное перепланирование",
		result:          result,
		timeAllocations: timeAllocations,
		scheduledCount:  len(tasks) - len(result.UnscheduledTasks),
		totalTasks:      len(tasks),
	}
	outcome.calendarSynced, outcome.calendarSyncFail, outcome.syncErrorDetail = h.syncGoogleCalendar(user, timeAllocations)
	h.sendScheduleOutcome(chatID, user, outcome)
}

func (h *BotHandler) executeInsertTask(chatID int64, user *models.User, taskID int64) {
	task, err := database.GetTaskByIDForUser(taskID, user.ID)
	if err != nil || task == nil {
		h.sendMessage(chatID, "Задача не найдена.")
		return
	}
	if task.Status == "completed" || task.Status == "cancelled" {
		h.sendMessage(chatID, "Эту задачу нельзя запланировать (уже завершена или отменена).")
		return
	}

	startDate := scheduleStartDate(user)
	existing, err := database.GetAllUserSchedulesFrom(user.ID, startDate)
	if err != nil {
		log.Printf("Error loading existing schedules: %v", err)
		h.sendMessage(chatID, "Ошибка чтения текущего расписания.")
		return
	}

	busy := h.fetchCalendarBusy(user, startDate, false)
	newDays, ok := scheduler.ScheduleTaskIntoExisting(user, *task, existing, startDate, busy)
	if !ok || len(newDays) == 0 {
		h.sendMessage(chatID, "⚠️ Не удалось вписать задачу в текущее расписание.\nСвободных слотов не хватает (дедлайн, загрузка или события в Google Calendar).\n\nПопробуйте «Перепланировать всё» — расписание будет пересобрано с нуля.")
		return
	}

	if err := database.SaveTaskSchedules(newDays); err != nil {
		log.Printf("Error saving incremental schedule: %v", err)
		h.sendMessage(chatID, "Ошибка сохранения расписания.")
		return
	}

	allSchedules, err := database.GetAllUserSchedulesFrom(user.ID, startDate)
	if err != nil {
		log.Printf("Error loading schedules after insert: %v", err)
		h.sendMessage(chatID, "Задача добавлена в БД, но не удалось обновить календарь.")
		return
	}
	newAllocations := scheduler.PlanTimeAllocations(user, newDays, startDate, busy)
	allAllocations := scheduler.PlanTimeAllocations(user, allSchedules, startDate, busy)

	outcome := scheduleOutcome{
		modeLabel:       "вписывание в текущее расписание",
		result:          &models.ScheduleResult{Success: true, Message: "Задача вписана в свободные слоты", DaySchedules: allSchedules},
		timeAllocations: allAllocations,
		scheduledCount:  1,
		totalTasks:      1,
	}
	outcome.calendarSynced, outcome.calendarSyncFail, outcome.syncErrorDetail = h.syncGoogleCalendarAppend(user, newAllocations)
	h.sendScheduleOutcome(chatID, user, outcome)
}

func shortenCalendarError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 200 {
		return msg[:200] + "…"
	}
	return msg
}

func (h *BotHandler) syncGoogleCalendar(user *models.User, allocations []models.SlotAllocation) (synced bool, failed bool, detail string) {
	if len(allocations) == 0 {
		return false, false, ""
	}

	ctx := context.Background()
	client, err := googlecal.ClientForUser(ctx, user.ID)
	if err != nil {
		log.Printf("google calendar client: %v", err)
		return false, true, shortenCalendarError(err)
	}
	if client == nil {
		return false, false, ""
	}

	if err := googlecal.SyncUserSchedule(ctx, client, user, allocations); err != nil {
		log.Printf("google calendar sync (full): %v", err)
		return true, true, shortenCalendarError(err)
	}
	return true, false, ""
}

func (h *BotHandler) syncGoogleCalendarAppend(user *models.User, allocations []models.SlotAllocation) (synced bool, failed bool, detail string) {
	if len(allocations) == 0 {
		return false, false, ""
	}

	ctx := context.Background()
	client, err := googlecal.ClientForUser(ctx, user.ID)
	if err != nil {
		log.Printf("google calendar client: %v", err)
		return false, true, shortenCalendarError(err)
	}
	if client == nil {
		return false, false, ""
	}

	if err := googlecal.AppendScheduleEvents(ctx, client, user, allocations); err != nil {
		log.Printf("google calendar sync (append): %v", err)
		return true, true, shortenCalendarError(err)
	}
	return true, false, ""
}

func (h *BotHandler) sendScheduleOutcome(chatID int64, user *models.User, o scheduleOutcome) {
	response := fmt.Sprintf("✅ Планирование завершено (%s).\n\n", o.modeLabel)
	if o.result != nil {
		response += fmt.Sprintf("📊 %s\n", o.result.Message)
	}
	response += fmt.Sprintf("📌 Запланировано задач: %d", o.scheduledCount)
	if o.totalTasks > 1 || o.modeLabel == "полное перепланирование" {
		response += fmt.Sprintf(" из %d", o.totalTasks)
	}
	response += "\n"

	if o.calendarSynced && !o.calendarSyncFail {
		if o.modeLabel == "вписывание в текущее расписание" {
			response += "📆 В Google Calendar добавлены события новой задачи (старые не тронуты).\n"
		} else {
			response += "📆 Google Calendar обновлён (учтены ваши события, расписание PlanBot перезаписано).\n"
		}
	} else if o.calendarSyncFail {
		response += "⚠️ Не удалось обновить Google Calendar.\n"
		if o.syncErrorDetail != "" {
			response += fmt.Sprintf("Причина: %s\n", o.syncErrorDetail)
		}
		response += "Проверьте /google_status или перезапустите бота после обновления.\n"
	}

	if o.result != nil && len(o.result.DaySchedules) > 0 {
		daySchedules := o.result.DaySchedules
		response += "\n📅 Расписание (по времени):\n\n"
		for i, daySchedule := range daySchedules {
			if i >= 7 {
				response += fmt.Sprintf("\n... и ещё %d дней", len(daySchedules)-7)
				break
			}
			response += formatDayScheduleWithTimes(daySchedule, user.DailyCapacity, o.timeAllocations)
		}
	}

	if o.result != nil && len(o.result.UnscheduledTasks) > 0 {
		response += fmt.Sprintf("\n\n⚠️ Не удалось запланировать %d задач(и)", len(o.result.UnscheduledTasks))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Сегодня", "view_today"),
			tgbotapi.NewInlineKeyboardButtonData("📆 Неделя", "view_week"),
		),
	)
	h.sendMessageWithReplyMarkup(chatID, response, &keyboard)
}

func planChoiceKeyboard(taskID int64, hasExisting bool) tgbotapi.InlineKeyboardMarkup {
	insertLabel := "📎 Вписать в расписание"
	if !hasExisting {
		insertLabel = "📎 Запланировать задачу"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(insertLabel, fmt.Sprintf("plan_insert:%d", taskID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Перепланировать всё", fmt.Sprintf("plan_rebuild:%d", taskID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏭ Позже", fmt.Sprintf("plan_skip:%d", taskID)),
		),
	)
}
