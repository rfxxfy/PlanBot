package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/adkhorst/planbot/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCommand — основная функция обработки команд
func HandleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.From.ID
	cmd := update.Message.Command()
	args := update.Message.CommandArguments()

	switch cmd {
	case "start":
		reply(bot, update.Message.Chat.ID, "Hello!")

	case "add_task":
		handleAddTask(bot, update.Message.Chat.ID, userID, args)

	case "my_tasks":
		handleMyTasks(bot, update.Message.Chat.ID, userID)

	case "today":
		handleTodayTasks(bot, update.Message.Chat.ID, userID)

	case "week":
		handleWeekTasks(bot, update.Message.Chat.ID, userID)

	case "update_task":
		handleUpdateTask(bot, update.Message.Chat.ID, args)

	case "delete_task":
		handleDeleteTask(bot, update.Message.Chat.ID, args)

	default:
		reply(bot, update.Message.Chat.ID, "Неизвестная команда. Попробуй /start")
	}
}

// handleAddTask - обрабатывает команду /add_task
// Формат: /add_task НАЗВАНИЕ | ОПИСАНИЕ | ГГГГ-ММ-ДД
func handleAddTask(bot *tgbotapi.BotAPI, chatID int64, userID int64, args string) {
	parts := strings.SplitN(args, "|", 2)
	if len(parts) < 2 {
		reply(bot, chatID, "Используй: /add_task НАЗВАНИЕ | ОПИСАНИЕ | ГГГГ-ММ-ДД")
		return
	}
	title := strings.TrimSpace(parts[0])
	desc := strings.TrimSpace(parts[1])
	dateStr := strings.TrimSpace(parts[2])

	dueDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		reply(bot, chatID, "Неверный формат даты. Используй: ГГГГ-ММ-ДД")
		return
	}

	err = database.CreateTaskWithDeadline(int64(userID), title, desc, 0, dueDate)
	if err != nil {
		log.Printf("Error creating task: %v", err)
		reply(bot, chatID, "Ошибка при добавлении задачи")
	} else {
		reply(bot, chatID, "Задача добавлена с дедлайном!")
	}
}

// handleMyTasks - обрабатывает команду /my_tasks
// Возвращает все задачи пользователя
func handleMyTasks(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	tasks, err := database.GetTasksByUserID(int64(userID))
	if err != nil {
		log.Printf("Error getting tasks: %v", err)
		reply(bot, chatID, "Ошибка при получении задач")
		return
	}
	if len(tasks) == 0 {
		reply(bot, chatID, "У вас нет задач.")
		return
	}

	var msg string
	for i, t := range tasks {
		due := "нет дедлайна"
		if t.Deadline != nil {
			due = t.Deadline.Format("02.01.2006")
		}
		msg += fmt.Sprintf("%d. [%s] %s (до %s)\n", i+1, t.Status, t.Title, due)
	}
	reply(bot, chatID, msg)
}

// handleTodayTasks - обрабатывает команду /today
// Возвращает задачи, дедлайн которых на сегодня
func handleTodayTasks(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	tasks, err := database.GetTasksForToday(int64(userID))
	if err != nil {
		log.Printf("Error getting today's tasks: %v", err)
		reply(bot, chatID, "Ошибка при получении задач на сегодня")
		return
	}
	if len(tasks) == 0 {
		reply(bot, chatID, "На сегодня задач нет.")
		return
	}

	var msg string
	for i, t := range tasks {
		due := "нет дедлайна"
		if t.Deadline != nil {
			due = t.Deadline.Format("15:04")
		}
		msg += fmt.Sprintf("%d. [%s] %s (до %s)\n", i+1, t.Status, t.Title, due)
	}
	reply(bot, chatID, msg)
}

// handleWeekTasks - обрабатывает команду /week
// Возвращает задачи, дедлайн которых в ближайшие 7 дней
func handleWeekTasks(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	tasks, err := database.GetTasksForWeek(int64(userID))
	if err != nil {
		log.Printf("Error getting week's tasks: %v", err)
		reply(bot, chatID, "Ошибка при получении задач на неделю")
		return
	}
	if len(tasks) == 0 {
		reply(bot, chatID, "На неделю задач нет.")
		return
	}

	var msg string
	for i, t := range tasks {
		due := "нет дедлайна"
		if t.Deadline != nil {
			due = t.Deadline.Format("02.01 15:04")
		}
		msg += fmt.Sprintf("%d. [%s] %s (до %s)\n", i+1, t.Status, t.Title, due)
	}
	reply(bot, chatID, msg)
}

// handleUpdateTask - обрабатывает команду /update_task
// Формат: /update_task ID поле новое_значение (/update_task 1 status done, /update_task 1 deadline 2025-12-20)
func handleUpdateTask(bot *tgbotapi.BotAPI, chatID int64, args string) {
	parts := strings.Fields(args)
	if len(parts) < 3 {
		reply(bot, chatID, "Используй: /update_task ID status done или /update_task ID deadline 2025-12-20 или /update_task ID priority 2")
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		reply(bot, chatID, "Неверный ID задачи")
		return
	}

	var newStatus *string
	var newPriority *int
	var newDueDate *time.Time

	i := 1
	for i < len(parts) {
		field := parts[i]
		value := parts[i+1]

		switch field {
		case "status":
			newStatus = &value
		case "priority":
			prio, err := strconv.Atoi(value)
			if err != nil {
				reply(bot, chatID, "Приоритет должен быть числом")
				return
			}
			newPriority = &prio
		case "deadline":
			due, err := time.Parse("2006-01-02", value)
			if err != nil {
				reply(bot, chatID, "Неверный формат даты. Используй: ГГГГ-ММ-ДД")
				return
			}
			newDueDate = &due
		default:
			reply(bot, chatID, "Неизвестное поле: "+field)
			return
		}
		i += 2
	}

	err = database.UpdateTask(id, newStatus, newPriority, newDueDate)
	if err != nil {
		log.Printf("Error updating task: %v", err)
		reply(bot, chatID, "Ошибка при обновлении задачи")
	} else {
		reply(bot, chatID, "Задача обновлена!")
	}
}

// handleDeleteTask — обрабатывает команду /delete_task
// Удаляет задачу по ID
func handleDeleteTask(bot *tgbotapi.BotAPI, chatID int64, args string) {
	id, err := strconv.ParseInt(args, 10, 64)
	if err != nil || args == "" {
		reply(bot, chatID, "Используй: /delete_task ID")
		return
	}

	// Удаляем задачу из БД
	err = database.DeleteTask(id)
	if err != nil {
		log.Printf("Error deleting task: %v", err)
		reply(bot, chatID, "Ошибка при удалении задачи")
	} else {
		reply(bot, chatID, "Задача удалена!")
	}
}

// reply - вспомогательная функция для отправки сообщения пользователю
func reply(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
