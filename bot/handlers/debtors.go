package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
)

func RegisterDebtors(b *ele.Bot, app *AppContext) {
	// Debtors button handled via common text router; callback buttons use "rp_sel" in payments.go
}

func showDebtors(c ele.Context, app *AppContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	debtors, err := app.SaleRepo.FindDebtors(ctx)
	if err != nil {
		return c.Send("❌ Qarzdorlar ro'yxatini yuklashda xatolik.")
	}

	if len(debtors) == 0 {
		return c.Send("✅ Qarzdorlar mavjud emas. Barcha to'lovlar qabul qilingan!")
	}

	var sb strings.Builder
	sb.WriteString("📋 Qarzdorlar ro'yxati\n\n")

	totalDue := 0.0
	menu := &ele.ReplyMarkup{}
	var rows []ele.Row

	for _, d := range debtors {
		dueStr := "muddat belgilanmagan"
		if d.DueDate != nil {
			dueStr = fmt.Sprintf("muddati ~%s", services.FormatDateLong(*d.DueDate))
		}

		buyerDisplay := d.BuyerName
		if buyerDisplay == "" {
			buyerDisplay = "Noma'lum"
		}

		sb.WriteString(fmt.Sprintf("👤 %s — %s — %s\n", buyerDisplay, services.FormatMoney(d.AmountDue), dueStr))
		totalDue += d.AmountDue

		btnText := fmt.Sprintf("💵 %s (%s)", buyerDisplay, services.FormatMoney(d.AmountDue))
		btn := menu.Data(btnText, "rp_sel", d.ID.Hex())
		rows = append(rows, menu.Row(btn))
	}

	sb.WriteString(fmt.Sprintf("\nJami qarz: %s", services.FormatMoney(totalDue)))
	menu.Inline(rows...)

	return c.Send(sb.String(), menu)
}

func showDueSoon(c ele.Context, app *AppContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	debtors, err := app.SaleRepo.FindDueSoon(ctx, app.DueSoonDays)
	if err != nil {
		return c.Send("❌ Ma'lumotlarni yuklashda xatolik.")
	}

	if len(debtors) == 0 {
		return c.Send(fmt.Sprintf("✅ Yaqin %d kunda to'lov muddati keladigan qarzlar yo'q.", app.DueSoonDays))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔔 Yaqin %d kunda to'lov muddati keladigan qarzlar:\n\n", app.DueSoonDays))

	menu := &ele.ReplyMarkup{}
	var rows []ele.Row

	for _, d := range debtors {
		dueStr := "muddat ko'rsatilmagan"
		if d.DueDate != nil {
			dueStr = services.FormatDateLong(*d.DueDate)
		}

		buyerDisplay := d.BuyerName
		if buyerDisplay == "" {
			buyerDisplay = "Noma'lum"
		}

		contactStr := ""
		if d.BuyerContact != "" {
			contactStr = fmt.Sprintf(" (📞 %s)", d.BuyerContact)
		}

		sb.WriteString(fmt.Sprintf("⚠️ %s%s\n   💰 Qarz: %s\n   📅 Muddat: %s\n\n",
			buyerDisplay, contactStr, services.FormatMoney(d.AmountDue), dueStr))

		btnText := fmt.Sprintf("💵 %s (To'lov)", buyerDisplay)
		btn := menu.Data(btnText, "rp_sel", d.ID.Hex())
		rows = append(rows, menu.Row(btn))
	}

	menu.Inline(rows...)
	return c.Send(sb.String(), menu)
}
