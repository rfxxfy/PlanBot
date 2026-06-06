package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/handlers"
	"github.com/adkhorst/planbot/health"
	"github.com/adkhorst/planbot/notifications"
)

// Version is set during build with -ldflags
var Version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Printf("PlanBot version %s starting...", Version)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Get bot token from environment
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	// Initialize database connection
	if err := database.InitDB(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.CloseDB()

	// Start health check server
	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8080"
	}
	healthServer := health.NewServer(healthPort, Version)
	healthServer.Start()
	log.Printf("Health check server running on port %s", healthPort)

	// Create bot instance
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = os.Getenv("BOT_DEBUG") == "true"
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Create bot handler
	handler := handlers.NewBotHandler(bot)

	// Start notifications
	notifications.StartNotifications(bot)

	// Configure updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Println("Bot is running... Press Ctrl+C to stop")

	// Handle incoming messages
	for update := range updates {
		handler.HandleUpdate(&update)
	}

	return nil
}
