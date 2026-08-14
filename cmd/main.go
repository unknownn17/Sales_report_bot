package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"telegram-sales-bot/bot"
	"telegram-sales-bot/bot/handlers"
	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/db"
	"telegram-sales-bot/repository"
	"telegram-sales-bot/services"
)

func main() {
	// 1. Load environment variables from .env file if available
	_ = godotenv.Load()

	// 2. Validate BOT_TOKEN
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("FATAL: BOT_TOKEN environment variable is required")
	}

	// 3. Validate ADMIN_IDS
	adminIDsStr := os.Getenv("ADMIN_IDS")
	if adminIDsStr == "" {
		log.Fatal("FATAL: ADMIN_IDS environment variable is required (comma-separated Telegram user IDs)")
	}

	adminIDs := make(map[int64]bool)
	for _, idStr := range strings.Split(adminIDsStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.Fatalf("FATAL: Invalid admin ID '%s': %v", idStr, err)
		}
		adminIDs[id] = true
	}

	if len(adminIDs) == 0 {
		log.Fatal("FATAL: At least one valid admin ID must be provided in ADMIN_IDS")
	}

	// 4. Mongo configuration
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "clothing_sales_bot"
	}

	dueSoonDays := 3
	if v := os.Getenv("DUE_SOON_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			dueSoonDays = d
		}
	}

	lowStockThreshold := 2
	if v := os.Getenv("LOW_STOCK_THRESHOLD"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d >= 0 {
			lowStockThreshold = d
		}
	}

	// 5. Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	log.Printf("Connecting to MongoDB at %s (Database: %s)...", mongoURI, dbName)
	client, err := db.Connect(ctx, mongoURI)
	if err != nil {
		log.Fatalf("FATAL: MongoDB connection failed: %v", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
	}()

	log.Println("MongoDB connection established and verified successfully.")
	database := db.GetDatabase(client, dbName)

	// 6. Ensure indexes
	if err := db.EnsureIndexes(ctx, database); err != nil {
		log.Fatalf("FATAL: Failed to create database indexes: %v", err)
	}
	log.Println("Database indexes ensured successfully.")

	// 7. Initialize repositories, services, and FSM session manager
	productRepo := repository.NewProductRepo(database)
	saleRepo := repository.NewSaleRepo(database)
	paymentRepo := repository.NewPaymentRepo(database)
	categoryRepo := repository.NewCategoryRepo(database)
	exchangeSvc := services.NewExchangeRateService()
	sessionMgr := state.NewManager()

	// Seed default categories if empty
	_ = categoryRepo.SeedDefaultCategories(ctx)

	appCtx := &handlers.AppContext{
		ProductRepo:       productRepo,
		SaleRepo:          saleRepo,
		PaymentRepo:       paymentRepo,
		CategoryRepo:      categoryRepo,
		SessionMgr:        sessionMgr,
		ExchangeSvc:       exchangeSvc,
		AdminIDs:          adminIDs,
		DueSoonDays:       dueSoonDays,
		LowStockThreshold: lowStockThreshold,
	}

	// 8. Create and start Telegram Bot
	teleBot, err := bot.NewBot(botToken)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize Telegram Bot: %v", err)
	}

	bot.SetupRoutes(teleBot, appCtx)

	log.Printf("Telegram Sales & Inventory Bot started successfully! Active admins: %d", len(adminIDs))
	teleBot.Start()
}
