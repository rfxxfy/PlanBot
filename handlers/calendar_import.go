package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/googlecal"
	"github.com/adkhorst/planbot/models"
)

func (h *BotHandler) handleCalendarImport(msg *tgbotapi.Message) {
	user, err := h.getUser(msg.From.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Ошибка получения пользователя")
		return
	}

	days := 30
	args := strings.TrimSpace(msg.CommandArguments())
	if args != "" {
		if v, err := strconv.Atoi(args); err == nil && v > 0 && v <= 180 {
			days = v
		}
	}

	client, err := googlecal.ClientForUser(context.Background(), user.ID)
	if err != nil {
		h.sendMessage(msg.Chat.ID, "Не удалось подключиться к Google Calendar.")
		return
	}
	if client == nil {
		h.sendMessage(msg.Chat.ID, "Google Calendar не подключен. Используйте /google_connect.")
		return
	}

	loc := time.Now().Location()
	if user.TimeZone != "" {
		if l, err := time.LoadLocation(user.TimeZone); err == nil {
			loc = l
		}
	}
	start := time.Now().In(loc)
	end := start.AddDate(0, 0, days)

	events, err := client.ListImportableEvents(context.Background(), "primary", start, end, user.TimeZone)
	if err != nil {
		log.Printf("calendar import list: %v", err)
		h.sendMessage(msg.Chat.ID, "Не удалось получить события из календаря.")
		return
	}

	imported := 0
	skipped := 0

	for _, ev := range events {
		linkedTaskID, err := database.GetTaskIDByGoogleEventID(user.ID, ev.EventID)
		if err != nil {
			continue
		}
		if linkedTaskID != nil {
			skipped++
			continue
		}

		hours := ev.End.Sub(ev.Start).Hours()
		if ev.AllDay {
			hours = user.DailyCapacity
			if hours <= 0 {
				hours = 8
			}
		}
		if hours < 0.25 {
			hours = 0.25
		}

		var deadline *time.Time
		d := ev.End
		deadline = &d

		task := &models.Task{
			UserID:        user.ID,
			Title:         ev.Title,
			HoursRequired: hours,
			Priority:      5,
			Deadline:      deadline,
		}
		if err := database.CreateTask(task); err != nil {
			continue
		}
		if err := database.SaveImportedCalendarLink(user.ID, ev.EventID, task.ID, ev.Start, ev.End); err != nil {
			continue
		}
		imported++
	}

	h.sendMessage(msg.Chat.ID, fmt.Sprintf("📥 Импорт из календаря завершён.\nИмпортировано задач: %d\nПропущено (уже связаны): %d\n\nДальше выполните /schedule или добавляйте точечно.", imported, skipped))
}
