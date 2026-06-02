package main

import (
	"log"
	"os"

	"github.com/adkhorst/planbot/database"
	"github.com/adkhorst/planbot/handlers"
	"github.com/adkhorst/planbot/health"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Version is set during build with -ldflags
var Version = "dev"

func main() {
	log.Printf("PlanBot version %s starting...", Version)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Initialize database connection
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
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

	// Get bot token from environment
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	// Create bot instance
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	bot.Debug = os.Getenv("BOT_DEBUG") == "true"
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Create bot handler
	handler := handlers.NewBotHandler(bot)

	// Configure updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Println("Bot is running... Press Ctrl+C to stop")

	// Handle incoming messages
	for update := range updates {
		handler.HandleUpdate(update)
	}
}
