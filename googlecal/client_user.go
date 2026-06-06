package googlecal

import (
	"context"

	"golang.org/x/oauth2"

	"github.com/adkhorst/planbot/database"
)

// ClientForUser builds a calendar client when the user has connected Google Calendar.
func ClientForUser(ctx context.Context, userID int64) (*Client, error) {
	storedTok, err := database.GetGoogleToken(userID)
	if err != nil {
		return nil, err
	}
	if storedTok == nil {
		return nil, nil
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}

	return NewWithStoredToken(ctx, cfg, storedTok, func(t *oauth2.Token) error {
		return database.SaveGoogleToken(userID, t)
	})
}
