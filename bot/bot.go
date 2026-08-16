package bot

import (
	"log"
	"time"

	"telegram-sales-bot/bot/handlers"

	ele "gopkg.in/telebot.v3"
)

// NewBot creates and configures a new Telegram bot instance
func NewBot(token string) (*ele.Bot, error) {
	pref := ele.Settings{
		Token:  token,
		Poller: &ele.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c ele.Context) {
			log.Printf("[Bot Error] %v", err)
		},
	}
	return ele.NewBot(pref)
}

// SetupRoutes registers all middlewares and handlers
func SetupRoutes(b *ele.Bot, app *handlers.AppContext) {
	// 1. Admin authorization middleware wrapping ALL handlers
	b.Use(func(next ele.HandlerFunc) ele.HandlerFunc {
		return func(c ele.Context) error {
			if c.Sender() == nil {
				return nil
			}
			adminID := c.Sender().ID
			if !app.AdminIDs[adminID] {
				log.Printf("Unauthorized access attempt by user ID %d (%s)", adminID, c.Sender().Username)
				return c.Send("Kechirasiz, siz ushbu botdan foydalanish huquqiga ega emassiz.")
			}
			return next(c)
		}
	})

	// 2. Start command
	b.Handle("/start", func(c ele.Context) error {
		app.SessionMgr.ClearSession(c.Sender().ID)
		return handlers.SendMainMenu(c)
	})

	// 3. Register all flow handler groups
	handlers.RegisterCancel(b, app)
	handlers.RegisterAddProduct(b, app)
	handlers.RegisterCategories(b, app)
	handlers.RegisterConfirmSale(b, app)
	handlers.RegisterPayments(b, app)
	handlers.RegisterReports(b, app)
	handlers.RegisterDebtors(b, app)
	handlers.RegisterStock(b, app)

	// 4. Catch-all for text and document messages during active flows
	b.Handle(ele.OnText, handlers.HandleTextInput(app))
	b.Handle(ele.OnDocument, handlers.HandleAddProductDoc(app))
	b.Handle(ele.OnPhoto, handlers.HandleAddProductPhoto(app))
}
