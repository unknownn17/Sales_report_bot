package handlers

import (
	"context"
	"log"
	"time"

	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/repository"
	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
)

type AppContext struct {
	ProductRepo       *repository.ProductRepo
	SaleRepo          *repository.SaleRepo
	PaymentRepo       *repository.PaymentRepo
	CategoryRepo      *repository.CategoryRepo
	SessionMgr        *state.Manager
	ExchangeSvc       *services.ExchangeRateService
	AdminIDs          map[int64]bool
	DueSoonDays       int
	LowStockThreshold int
}

func SendMainMenu(c ele.Context) error {
	menu := &ele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text("➕ Mahsulot qo'shish"), menu.Text("💰 Sotuvni tasdiqlash")),
		menu.Row(menu.Text("📦 Ombor / Zaxira"), menu.Text("📁 Kategoriyalar")),
		menu.Row(menu.Text("📊 Bugungi hisobot"), menu.Text("📈 Haftalik/Oylik hisobot")),
		menu.Row(menu.Text("📋 Qarzdorlar"), menu.Text("🔔 Muddati yaqin")),
	)
	return c.Send("Asosiy menyu:", menu)
}

func RegisterCancel(b *ele.Bot, app *AppContext) {
	b.Handle("/cancel", func(c ele.Context) error {
		return HandleCancel(c, app)
	})
}

func HandleCancel(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	sess := app.SessionMgr.GetSession(adminID)

	// Rollback reserved stock if cancelling mid-sale
	if sess.Flow == state.FlowConfirmSale && app.SessionMgr.GetDataBool(adminID, "stock_reserved") {
		pid := app.SessionMgr.GetDataObjectID(adminID, "sale_product_id")
		size := app.SessionMgr.GetDataString(adminID, "sale_size")
		qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")

		if !pid.IsZero() && qty > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := app.ProductRepo.IncrementStock(ctx, pid, size, qty); err != nil {
				log.Printf("Failed to rollback stock for admin %d: %v", adminID, err)
			} else {
				log.Printf("Successfully rolled back %d items of size '%s' for product %s", qty, size, pid.Hex())
			}
		}
	}

	app.SessionMgr.ClearSession(adminID)
	c.Send("❌ Amal bekor qilindi.")
	return SendMainMenu(c)
}

func HandleTextInput(app *AppContext) ele.HandlerFunc {
	return func(c ele.Context) error {
		adminID := c.Sender().ID
		if !app.AdminIDs[adminID] {
			return nil
		}

		text := c.Message().Text
		if text == "/cancel" {
			return HandleCancel(c, app)
		}

		sess := app.SessionMgr.GetSession(adminID)
		switch sess.Flow {
		case state.FlowConfirmSale:
			return handleConfirmSaleText(c, app, sess)
		case state.FlowRecordPayment:
			return handleRecordPaymentText(c, app, sess)
		case state.FlowCustomReport:
			return handleCustomReportText(c, app, sess)
		case state.FlowCategoryManage:
			return handleCategoryManageText(c, app, sess)
		default:
			return handleMenuButton(c, app)
		}
	}
}

func handleMenuButton(c ele.Context, app *AppContext) error {
	text := c.Message().Text
	switch text {
	case "➕ Mahsulot qo'shish":
		return startAddProduct(c, app)
	case "💰 Sotuvni tasdiqlash":
		return startConfirmSale(c, app)
	case "📁 Kategoriyalar":
		return showCategoriesMenu(c, app)
	case "📊 Bugungi hisobot":
		return showTodayReport(c, app)
	case "📈 Haftalik/Oylik hisobot":
		return showPeriodReportMenu(c, app)
	case "📦 Ombor / Zaxira":
		return startStockHierarchy(c, app)
	case "📋 Qarzdorlar":
		return showDebtors(c, app)
	case "🔔 Muddati yaqin":
		return showDueSoon(c, app)
	default:
		return SendMainMenu(c)
	}
}
