package notifications

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/models"
)

var (
	bot      *tgbotapi.BotAPI
	stopChan = make(chan bool)
)

// StartNotifications - запускает фоновую горутину, которая периодически проверяет дедлайны
func StartNotifications(b *tgbotapi.BotAPI) {
	bot = b
	go func() {
		ticker := time.NewTicker(30 * time.Minute) // Проверяем раз в полчаса
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				SendReminders()
			case <-stopChan:
				log.Println("Notification service stopped")
				return
			}
		}
	}()
	log.Println("Reminder notifications started")
}

// StopNotifications - останавливает фоновую проверку уведомлений
func StopNotifications() {
	stopChan <- true
}

// SendReminders - основная функция, которая проверяет задачи и отправляет уведомления
func SendReminders() {
	// Получаем всех пользователей, чтобы учитывать их таймзоны
	users, err := getAllUsers()
	if err != nil {
		log.Printf("Error fetching users for reminders: %v", err)
		return
	}

	for _, user := range users {
		sendUserReminders(user)
	}
}

func sendUserReminders(user models.User) {
	loc := time.UTC
	if user.TimeZone != "" {
		if l, err := time.LoadLocation(user.TimeZone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)

	// Use a small window to avoid duplicate notifications with the 30-min ticker
	if now.Minute() >= 30 {
		return
	}

	// 1. Задачи, дедлайн которых завтра
	if now.Hour() == 9 {
		tomorrow := now.AddDate(0, 0, 1)
		tomorrowStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc)
		tomorrowEnd := tomorrowStart.Add(24 * time.Hour).Add(-time.Second)

		soonTasks, err := getTasksByDeadlineRange(user.ID, tomorrowStart, tomorrowEnd)
		if err == nil {
			for _, t := range soonTasks {
				sendNotification(user.TelegramID, fmt.Sprintf("⏰ Напоминаю: задача \"%s\" истекает завтра (%s)", t.Title, t.Deadline.Format("02.01.2006")))
			}
		}

		// 2. Задачи, дедлайн которых сегодня
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		todayEnd := todayStart.Add(24 * time.Hour).Add(-time.Second)

		todayTasks, err := getTasksByDeadlineRange(user.ID, todayStart, todayEnd)
		if err == nil {
			for _, t := range todayTasks {
				sendNotification(user.TelegramID, fmt.Sprintf("⚠️ Задача \"%s\" сегодня дедлайн! (%s)", t.Title, t.Deadline.Format("02.01.2006")))
			}
		}
	}

	// 3. Просроченные задачи
	if now.Hour() == 10 {
		overdueTasks, err := getOverdueTasks(user.ID, now)
		if err == nil {
			for _, t := range overdueTasks {
				sendNotification(user.TelegramID, fmt.Sprintf("❌ Задача \"%s\" просрочена! (была до %s)", t.Title, t.Deadline.Format("02.01.2006")))
			}
		}
	}
}

func getAllUsers() ([]models.User, error) {
	query := `SELECT id, telegram_id, time_zone FROM users`
	rows, err := database.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.TimeZone); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func getTasksByDeadlineRange(userID int64, start, end time.Time) ([]models.Task, error) {
	query := `
		SELECT id, title, deadline
		FROM tasks
		WHERE user_id = $1 AND deadline BETWEEN $2 AND $3 AND status NOT IN ('completed', 'cancelled')
		ORDER BY deadline ASC`

	rows, err := database.DB.Query(query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Deadline); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func getOverdueTasks(userID int64, now time.Time) ([]models.Task, error) {
	query := `
		SELECT id, title, deadline
		FROM tasks
		WHERE user_id = $1 AND deadline < $2 AND status NOT IN ('completed', 'cancelled')
		ORDER BY deadline ASC`

	rows, err := database.DB.Query(query, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Deadline); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func sendNotification(telegramID int64, message string) {
	msg := tgbotapi.NewMessage(telegramID, message)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending notification to user %d: %v", telegramID, err)
	}
}
