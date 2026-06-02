package database

import "log"

// EnsureSchema applies idempotent schema updates for existing databases.
func EnsureSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS google_calendar_events (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			google_event_id VARCHAR(255) NOT NULL,
			task_id BIGINT REFERENCES tasks(id) ON DELETE SET NULL,
			source VARCHAR(50) NOT NULL DEFAULT 'planbot',
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_google_calendar_events_user_id ON google_calendar_events(user_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_google_calendar_events_user_event ON google_calendar_events(user_id, google_event_id)`,
		`ALTER TABLE google_calendar_events ADD COLUMN IF NOT EXISTS source VARCHAR(50) NOT NULL DEFAULT 'planbot'`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}

	log.Println("Database schema ensured (google_calendar_events)")
	return nil
}
