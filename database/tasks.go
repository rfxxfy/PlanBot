package database

import (
	"time"
)

// Task - структура для представления задачи
type Task struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"`
	Status      string     `json:"status"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// CreateTaskWithDeadline - создаёт новую задачу с обязательным дедлайном
func CreateTaskWithDeadline(userID int64, title, description string, priority int, deadline time.Time) error {
	query := `
		INSERT INTO tasks (user_id, title, description, priority, deadline)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := DB.Exec(query, userID, title, description, priority, deadline)
	return err
}

// GetTaskByID - возвращает задачу по её ID
func GetTaskByID(id int64) (*Task, error) {
	query := `SELECT id, user_id, title, description, priority, status, deadline, created_at, updated_at, completed_at FROM tasks WHERE id = $1`
	row := DB.QueryRow(query, id)

	task := &Task{}
	err := row.Scan(&task.ID, &task.UserID, &task.Title, &task.Description, &task.Priority, &task.Status, &task.Deadline, &task.CreatedAt, &task.UpdatedAt, &task.CompletedAt)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetTasksByUserID - возвращает все задачи конкретного пользователя, отсортированные по дате создания
func GetTasksByUserID(userID int64) ([]*Task, error) {
	query := `SELECT id, title, description, priority, status, deadline, created_at FROM tasks WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Priority, &task.Status, &task.Deadline, &task.CreatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetTasksForToday - возвращает задачи, дедлайн которых на сегодня
func GetTasksForToday(userID int64) ([]*Task, error) {
	query := `
		SELECT id, title, description, priority, status, deadline
		FROM tasks
		WHERE user_id = $1 AND deadline::date = CURRENT_DATE
		ORDER BY deadline ASC`
	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Priority, &task.Status, &task.Deadline)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetTasksForWeek - возвращает задачи, дедлайн которых в ближайшие 7 дней
func GetTasksForWeek(userID int64) ([]*Task, error) {
	query := `
		SELECT id, title, description, priority, status, deadline
		FROM tasks
		WHERE user_id = $1 AND deadline BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '7 days'
		ORDER BY deadline ASC`
	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Priority, &task.Status, &task.Deadline)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// UpdateTask - обновляет поля задачи (статус, приоритет, дедлайн), можно передать nil для пропуска
func UpdateTask(id int64, status *string, priority *int, deadline *time.Time) error {
	query := `
		UPDATE tasks
		SET status = COALESCE($1, status),
		    priority = COALESCE($2, priority),
		    deadline = COALESCE($3, deadline),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $4`
	_, err := DB.Exec(query, status, priority, deadline, id)
	return err
}
