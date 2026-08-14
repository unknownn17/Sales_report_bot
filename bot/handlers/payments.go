package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/models"
	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	btnRPSelectSale = (&ele.ReplyMarkup{}).Data("", "rp_sel")
)

func RegisterPayments(b *ele.Bot, app *AppContext) {
	b.Handle(&btnRPSelectSale, func(c ele.Context) error {
		saleIDHex := c.Callback().Data
		saleID, err := primitive.ObjectIDFromHex(saleIDHex)
		if err != nil {
			return nil
		}

		adminID := c.Sender().ID
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		sale, err := app.SaleRepo.FindSaleByID(ctx, saleID)
		if err != nil || sale == nil {
			return c.Send("❌ Sotuv topilmadi.")
		}

		app.SessionMgr.SetFlow(adminID, state.FlowRecordPayment, state.PaymentStepEnterAmount)
		app.SessionMgr.SetData(adminID, "rp_sale_id", saleID)
		app.SessionMgr.SetData(adminID, "rp_amount_due", sale.AmountDue)
		app.SessionMgr.SetData(adminID, "rp_buyer_name", sale.BuyerName)

		return c.Send(fmt.Sprintf("👤 Xaridor: %s\n⏳ Qolgan qarz: %s\n\nQabul qilingan to'lov summasini kiriting (Bekor qilish: /cancel):",
			sale.BuyerName, services.FormatMoney(sale.AmountDue)))
	})
}

func handleRecordPaymentText(c ele.Context, app *AppContext, sess *state.Session) error {
	adminID := c.Sender().ID
	text := strings.TrimSpace(c.Message().Text)

	if sess.Step == state.PaymentStepEnterAmount {
		cleaned := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, text)

		amount, err := strconv.ParseFloat(cleaned, 64)
		if err != nil || amount <= 0 {
			return c.Send("Iltimos, to'g'ri musbat summa kiriting:")
		}

		saleID := app.SessionMgr.GetDataObjectID(adminID, "rp_sale_id")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		sale, err := app.SaleRepo.FindSaleByID(ctx, saleID)
		if err != nil || sale == nil {
			app.SessionMgr.ClearSession(adminID)
			return c.Send("❌ Sotuv topilmadi.")
		}

		if amount > sale.AmountDue {
			return c.Send(fmt.Sprintf("Kiritilgan summa qarz miqdoridan (%s) katta! Qaytadan kiriting:", services.FormatMoney(sale.AmountDue)))
		}

		newPaid := sale.AmountPaid + amount
		newDue := sale.AmountDue - amount
		newStatus := models.PaymentStatusPartiallyPaid
		if newDue <= 0 {
			newDue = 0
			newStatus = models.PaymentStatusPaid
		}

		if err := app.SaleRepo.UpdateSalePayment(ctx, saleID, newPaid, newDue, newStatus); err != nil {
			log.Printf("UpdateSalePayment error: %v", err)
			return c.Send("❌ To'lovni saqlashda xatolik yuz berdi.")
		}

		payment := &models.Payment{
			SaleID: saleID,
			Amount: amount,
			PaidAt: time.Now(),
			Note:   "Qarz to'lovi",
		}
		if err := app.PaymentRepo.InsertPayment(ctx, payment); err != nil {
			log.Printf("InsertPayment error: %v", err)
		}

		app.SessionMgr.ClearSession(adminID)

		statusMsg := fmt.Sprintf("Qolgan qarz: %s", services.FormatMoney(newDue))
		if newStatus == models.PaymentStatusPaid {
			statusMsg = "🎉 Qarz to'liq yopildi!"
		}

		c.Send(fmt.Sprintf("✅ To'lov qabul qilindi: %s\n👤 Xaridor: %s\n%s",
			services.FormatMoney(amount), sale.BuyerName, statusMsg))
		return SendMainMenu(c)
	}

	return nil
}
