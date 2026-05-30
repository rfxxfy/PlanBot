package notifications

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/adkhorst/planbot/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Reminder - структура для хранения информации о напоминании
type Reminder struct {
	TaskID   int64
	UserID   int64
	Title    string
	Deadline time.Time
	SentOnce bool // для однократного уведомления
}

var (
	mu       sync.Mutex
	bot      *tgbotapi.BotAPI
	stopChan = make(chan bool)
)

// StartNotifications - запускает фоновую горутину, которая каждые 10 минут проверяет дедлайны
func StartNotifications(b *tgbotapi.BotAPI) {
	bot = b
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // проверять каждые 10 минут
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
	now := time.Now()

	// 1. Задачи, дедлайн которых завтра
	tomorrowStart := now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	tomorrowEnd := tomorrowStart.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	soonTasks, err := getTasksByDeadlineRange(tomorrowStart, tomorrowEnd)
	if err != nil {
		log.Printf("Error fetching tomorrow tasks: %v", err)
	} else {
		for _, t := range soonTasks {
			sendNotification(t, fmt.Sprintf("⏰ Напоминаю: задача \"%s\" истекает завтра (%s)", t.Title, t.Deadline.Format("02.01.2006")))
		}
	}

	// 2. Задачи, дедлайн которых сегодня, в 9:00
	if now.Hour() == 9 && now.Minute() < 5 { // в течение первых 5 минут часа
		todayStart := now.Truncate(24 * time.Hour)
		todayEnd := todayStart.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

		todayTasks, err := getTasksByDeadlineRange(todayStart, todayEnd)
		if err != nil {
			log.Printf("Error fetching today tasks: %v", err)
		} else {
			for _, t := range todayTasks {
				sendNotification(t, fmt.Sprintf("⚠️ Задача \"%s\" сегодня дедлайн! (%s)", t.Title, t.Deadline.Format("02.01.2006")))
			}
		}
	}

	// 3. Просроченные задачи (не выполнены)
	overdueTasks, err := getOverdueTasks()
	if err != nil {
		log.Printf("Error fetching overdue tasks: %v", err)
	} else {
		for _, t := range overdueTasks {
			sendNotification(t, fmt.Sprintf("❌ Задача \"%s\" просрочена! (была до %s)", t.Title, t.Deadline.Format("02.01.2006")))
		}
	}
}

// getTasksByDeadlineRange - возвращает задачи в диапазоне дат (от start до end), кроме выполненных
func getTasksByDeadlineRange(start, end time.Time) ([]*database.Task, error) {
	query := `
		SELECT id, user_id, title, deadline
		FROM tasks
		WHERE deadline BETWEEN $1 AND $2 AND status != 'completed'
		ORDER BY deadline ASC`

	rows, err := database.DB.Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*database.Task
	for rows.Next() {
		task := &database.Task{}
		err := rows.Scan(&task.ID, &task.UserID, &task.Title, &task.Deadline)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// getOverdueTasks - возвращает просроченные задачи (дедлайн раньше текущего времени)
func getOverdueTasks() ([]*database.Task, error) {
	query := `
		SELECT id, user_id, title, deadline
		FROM tasks
		WHERE deadline < CURRENT_TIMESTAMP AND status != 'completed'
		ORDER BY deadline ASC`

	rows, err := database.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*database.Task
	for rows.Next() {
		task := &database.Task{}
		err := rows.Scan(&task.ID, &task.UserID, &task.Title, &task.Deadline)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// sendNotification - отправляет уведомление пользователю в Telegram
func sendNotification(task *database.Task, message string) {
	msg := tgbotapi.NewMessage(task.UserID, message)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending notification to user %d: %v", task.UserID, err)
	} else {
		log.Printf("Notification sent to user %d: %s", task.UserID, message)
	}
}
