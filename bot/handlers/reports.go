package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
)

var (
	btnRepToday   = (&ele.ReplyMarkup{}).Data("📊 Bugun", "rep_today")
	btnRepWeekly  = (&ele.ReplyMarkup{}).Data("📅 Bu hafta", "rep_weekly")
	btnRepMonthly = (&ele.ReplyMarkup{}).Data("📅 Bu oy", "rep_monthly")
	btnRepCustom  = (&ele.ReplyMarkup{}).Data("🗓 Boshqa sana", "rep_custom")
)

func RegisterReports(b *ele.Bot, app *AppContext) {
	b.Handle(&btnRepToday, func(c ele.Context) error {
		start, end := services.TodayRange()
		return renderProfitReport(c, app, start, end, "Bugungi hisobot", "Kunning yetakchisi")
	})

	b.Handle(&btnRepWeekly, func(c ele.Context) error {
		start, end := services.ThisWeekRange()
		return renderProfitReport(c, app, start, end, "Haftalik hisobot", "Haftaning eng ko'p sotilganlari")
	})

	b.Handle(&btnRepMonthly, func(c ele.Context) error {
		start, end := services.ThisMonthRange()
		return renderProfitReport(c, app, start, end, "Oylik hisobot", "Oyning eng ko'p sotilganlari")
	})

	b.Handle(&btnRepCustom, func(c ele.Context) error {
		adminID := c.Sender().ID
		app.SessionMgr.SetFlow(adminID, state.FlowCustomReport, state.ReportStepEnterStartDate)
		return c.Send("🗓 Boshlang'ich sanani kiriting (masalan: 01.08.2026):\nBekor qilish: /cancel")
	})
}

func showTodayReport(c ele.Context, app *AppContext) error {
	start, end := services.TodayRange()
	return renderProfitReport(c, app, start, end, "Bugungi hisobot", "Kunning yetakchisi")
}

func showPeriodReportMenu(c ele.Context, app *AppContext) error {
	menu := &ele.ReplyMarkup{}
	menu.Inline(
		menu.Row(btnRepToday, btnRepWeekly),
		menu.Row(btnRepMonthly, btnRepCustom),
	)
	return c.Send("📈 Qaysi davr bo'yicha hisobot kerak?", menu)
}

func handleCustomReportText(c ele.Context, app *AppContext, sess *state.Session) error {
	adminID := c.Sender().ID
	text := strings.TrimSpace(c.Message().Text)

	if sess.Step == state.ReportStepEnterStartDate {
		t, err := parseDateInput(text)
		if err != nil {
			return c.Send("Iltimos, to'g'ri sanani kiriting (format: DD.MM.YYYY, masalan: 01.08.2026):")
		}
		app.SessionMgr.SetData(adminID, "custom_rep_start", t)
		app.SessionMgr.SetStep(adminID, state.ReportStepEnterEndDate)
		return c.Send("🗓 Tugash sanasini kiriting (masalan: 15.08.2026):")
	}

	if sess.Step == state.ReportStepEnterEndDate {
		endT, err := parseDateInput(text)
		if err != nil {
			return c.Send("Iltimos, to'g'ri sanani kiriting (format: DD.MM.YYYY, masalan: 15.08.2026):")
		}

		startVal, ok := app.SessionMgr.GetData(adminID, "custom_rep_start")
		if !ok {
			app.SessionMgr.ClearSession(adminID)
			return SendMainMenu(c)
		}
		startT := startVal.(time.Time)
		// Set end date to end of day
		endT = endT.Add(24 * time.Hour)

		app.SessionMgr.ClearSession(adminID)
		title := fmt.Sprintf("Hisobot (%s - %s)", services.FormatDateShort(startT), services.FormatDateShort(endT.Add(-24*time.Hour)))
		return renderProfitReport(c, app, startT, endT, title, "Eng ko'p sotilgan mahsulotlar")
	}

	return nil
}

func parseDateInput(text string) (time.Time, error) {
	formats := []string{"02.01.2006", "02/01/2006", "2006-01-02", "02-01-2006"}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, text, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid format")
}

func renderProfitReport(c ele.Context, app *AppContext, start, end time.Time, periodName, topLeaderTitle string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rep, err := app.SaleRepo.ProfitReport(ctx, start, end)
	if err != nil {
		return c.Send("❌ Hisobotni hisoblashda xatolik yuz berdi.")
	}

	dateSubtitle := services.FormatDateLong(start)
	if periodName == "Haftalik hisobot" || periodName == "Oylik hisobot" {
		dateSubtitle = fmt.Sprintf("%s — %s", services.FormatDateShort(start), services.FormatDateShort(end.Add(-time.Second)))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s — %s\n\n", periodName, dateSubtitle))
	sb.WriteString(fmt.Sprintf("💰 Olingan foyda:        %s\n", services.FormatMoney(rep.ReceivedProfit)))
	sb.WriteString(fmt.Sprintf("⏳ Kutilayotgan foyda:      %s\n", services.FormatMoney(rep.PendingProfit)))
	sb.WriteString("────────────────────────\n")
	sb.WriteString(fmt.Sprintf("📦 Sotilgan mahsulotlar: %d dona\n", rep.TotalSold))
	sb.WriteString(fmt.Sprintf("💵 Naqd: %d | 💳 Karta: %d | 📝 Nasiya: %d\n", rep.CashCount, rep.CardCount, rep.CreditCount))

	if len(rep.TopProducts) > 0 {
		sb.WriteString("\n")
		if len(rep.TopProducts) == 1 || periodName == "Bugungi hisobot" {
			sb.WriteString(fmt.Sprintf("🏆 %s: %s (%d dona)\n", topLeaderTitle, rep.TopProducts[0].ProductName, rep.TopProducts[0].TotalQty))
		} else {
			sb.WriteString(fmt.Sprintf("🏆 %s:\n", topLeaderTitle))
			for i, tp := range rep.TopProducts {
				sb.WriteString(fmt.Sprintf("  %d. %s — %d dona\n", i+1, tp.ProductName, tp.TotalQty))
			}
		}
	} else if rep.TotalSold == 0 {
		sb.WriteString("\nℹ️ Ushbu davrda hali sotuvlar amalga oshirilmagan.")
	}

	if c.Callback() != nil {
		return c.Send(sb.String())
	}
	return c.Send(sb.String())
}
