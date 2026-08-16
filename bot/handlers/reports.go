package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/models"
	"telegram-sales-bot/repository"
	"telegram-sales-bot/services"

	"github.com/xuri/excelize/v2"
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
		return renderProfitReportText(c, app, start, end, "Bugungi hisobot", "Kunning yetakchisi")
	})

	b.Handle(&btnRepWeekly, func(c ele.Context) error {
		start, end := services.ThisWeekRange()
		return sendReportExcel(c, app, start, end, "Haftalik hisobot")
	})

	b.Handle(&btnRepMonthly, func(c ele.Context) error {
		start, end := services.ThisMonthRange()
		return sendReportExcel(c, app, start, end, "Oylik hisobot")
	})

	b.Handle(&btnRepCustom, func(c ele.Context) error {
		adminID := c.Sender().ID
		app.SessionMgr.SetFlow(adminID, state.FlowCustomReport, state.ReportStepEnterStartDate)
		return c.Send("🗓 Boshlang'ich sanani kiriting (masalan: 01.08.2026):\nBekor qilish: /cancel")
	})
}

func showTodayReport(c ele.Context, app *AppContext) error {
	start, end := services.TodayRange()
	return renderProfitReportText(c, app, start, end, "Bugungi hisobot", "Kunning yetakchisi")
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
		endT = endT.Add(24 * time.Hour)

		app.SessionMgr.ClearSession(adminID)
		title := fmt.Sprintf("Hisobot (%s - %s)", services.FormatDateShort(startT), services.FormatDateShort(endT.Add(-24*time.Hour)))
		return sendReportExcel(c, app, startT, endT, title)
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

// renderProfitReportText sends today's report as a text message (small enough for Telegram).
func renderProfitReportText(c ele.Context, app *AppContext, start, end time.Time, periodName, topLeaderTitle string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rep, err := app.SaleRepo.ProfitReport(ctx, start, end)
	if err != nil {
		return c.Send("❌ Hisobotni hisoblashda xatolik yuz berdi.")
	}

	dateSubtitle := services.FormatDateLong(start)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s — %s\n\n", periodName, dateSubtitle))
	sb.WriteString(fmt.Sprintf("💰 Olingan foyda:        %s\n", services.FormatMoney(rep.ReceivedProfit)))
	sb.WriteString(fmt.Sprintf("⏳ Kutilayotgan foyda:      %s\n", services.FormatMoney(rep.PendingProfit)))
	sb.WriteString("────────────────────────\n")
	sb.WriteString(fmt.Sprintf("📦 Sotilgan mahsulotlar: %d dona\n", rep.TotalSold))
	sb.WriteString(fmt.Sprintf("💵 Naqd: %d | 💳 Karta: %d | 📝 Nasiya: %d\n", rep.CashCount, rep.CardCount, rep.CreditCount))

	if len(rep.TopProducts) > 0 {
		sb.WriteString("\n")
		if len(rep.TopProducts) == 1 {
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

	return c.Send(sb.String())
}

// sendReportExcel generates an Excel report file and sends it as a document.
func sendReportExcel(c ele.Context, app *AppContext, start, end time.Time, periodName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rep, err := app.SaleRepo.ProfitReport(ctx, start, end)
	if err != nil {
		return c.Send("❌ Hisobotni hisoblashda xatolik yuz berdi.")
	}

	// Fetch detailed sales
	var sales []models.Sale
	var salesErr string
	result, err := app.SaleRepo.FindSalesInRange(ctx, start, end)
	if err != nil {
		salesErr = fmt.Sprintf("Sotuvlar yuklanmadi: %v", err)
	} else {
		sales = result
	}

	// Fetch all products for detail enrichment
	var productMap map[string]*models.Product
	var productsErr string
	productMap = make(map[string]*models.Product)
	products, err := app.ProductRepo.FindAllActiveWithStock(ctx)
	if err != nil {
		productsErr = fmt.Sprintf("Mahsulotlar yuklanmadi: %v", err)
	} else {
		for i := range products {
			productMap[products[i].ID.Hex()] = &products[i]
		}
	}

	// Generate Excel file
	tmpPath, err := generateReportExcel(rep, sales, salesErr, productMap, productsErr, periodName, start, end)
	if err != nil {
		return c.Send("❌ Excel hisobot yaratishda xatolik.")
	}

	// Send the Excel file with a proper filename
	filename := fmt.Sprintf("%s.xlsx", periodName)
	doc := &ele.Document{
		File:     ele.FromDisk(tmpPath),
		FileName: filename,
		MIME:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	sendErr := c.Send(doc)

	// Clean up after sending
	os.Remove(tmpPath)

	return sendErr
}

// generateReportExcel creates a styled Excel file with summary, detailed sales, and product catalog sheets.
func generateReportExcel(rep *repository.ProfitReportResult, sales []models.Sale, salesErr string, productMap map[string]*models.Product, productsErr string, periodName string, start, end time.Time) (string, error) {
	f := excelize.NewFile()
	defer f.Close()

	// ── Styles ──────────────────────────────────────────────────────────────
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#2F5496"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	subtitleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 11, Italic: true, Color: "#404040"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#4472C4"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#2F5496", Style: 1},
			{Type: "right", Color: "#2F5496", Style: 1},
			{Type: "top", Color: "#2F5496", Style: 1},
			{Type: "bottom", Color: "#2F5496", Style: 1},
		},
	})
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#D9D9D9", Style: 1},
			{Type: "right", Color: "#D9D9D9", Style: 1},
			{Type: "top", Color: "#D9D9D9", Style: 1},
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
		},
	})
	altRowStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#F2F7FB"}},
		Border: []excelize.Border{
			{Type: "left", Color: "#D9D9D9", Style: 1},
			{Type: "right", Color: "#D9D9D9", Style: 1},
			{Type: "top", Color: "#D9D9D9", Style: 1},
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
		},
	})
	summaryLabelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
		},
	})
	summaryValueStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
		},
	})

	// ── Sheet 1: Summary (Xulosa) ───────────────────────────────────────────
	summarySheet := "Xulosa"
	f.SetSheetName("Sheet1", summarySheet)

	// Title row
	f.SetCellValue(summarySheet, "A1", periodName)
	f.MergeCell(summarySheet, "A1", "C1")
	f.SetCellStyle(summarySheet, "A1", "C1", titleStyle)
	f.SetRowHeight(summarySheet, 1, 35)

	// Date range subtitle
	dateRange := fmt.Sprintf("%s — %s", services.FormatDateShort(start), services.FormatDateShort(end.Add(-time.Second)))
	f.SetCellValue(summarySheet, "A2", dateRange)
	f.MergeCell(summarySheet, "A2", "C2")
	f.SetCellStyle(summarySheet, "A2", "C2", subtitleStyle)

	// Summary table
	row := 4
	f.SetCellValue(summarySheet, fmt.Sprintf("A%d", row), "Ko'rsatkich")
	f.SetCellValue(summarySheet, fmt.Sprintf("B%d", row), "Qiymat")
	f.SetCellStyle(summarySheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), headerStyle)

	type summaryItem struct {
		label string
		value interface{}
	}
	totalProfit := rep.ReceivedProfit + rep.PendingProfit
	summaryData := []summaryItem{
		{"Jami foyda", services.FormatMoneyRaw(totalProfit) + " so'm"},
		{"Olingan foyda", services.FormatMoneyRaw(rep.ReceivedProfit) + " so'm"},
		{"Kutilayotgan foyda", services.FormatMoneyRaw(rep.PendingProfit) + " so'm"},
		{"─────────────────", ""},
		{"Sotilgan mahsulotlar", fmt.Sprintf("%d dona", rep.TotalSold)},
		{"Jami tushum", services.FormatMoneyRaw(calcTotalRevenue(sales)) + " so'm"},
		{"─────────────────", ""},
		{"Naqd to'lovlar", fmt.Sprintf("%d ta", rep.CashCount)},
		{"Karta to'lovlar", fmt.Sprintf("%d ta", rep.CardCount)},
		{"Nasiya to'lovlar", fmt.Sprintf("%d ta", rep.CreditCount)},
	}

	for i, item := range summaryData {
		r := row + 1 + i
		f.SetCellValue(summarySheet, fmt.Sprintf("A%d", r), item.label)
		f.SetCellValue(summarySheet, fmt.Sprintf("B%d", r), item.value)
		st := dataStyle
		if i%2 == 1 {
			st = altRowStyle
		}
		f.SetCellStyle(summarySheet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), summaryLabelStyle)
		f.SetCellStyle(summarySheet, fmt.Sprintf("B%d", r), fmt.Sprintf("B%d", r), summaryValueStyle)
		_ = st
	}

	// Top products section
	if len(rep.TopProducts) > 0 {
		topStart := row + len(summaryData) + 3
		f.SetCellValue(summarySheet, fmt.Sprintf("A%d", topStart), "🏆 Eng ko'p sotilgan mahsulotlar")
		f.MergeCell(summarySheet, fmt.Sprintf("A%d", topStart), fmt.Sprintf("C%d", topStart))
		f.SetCellStyle(summarySheet, fmt.Sprintf("A%d", topStart), fmt.Sprintf("C%d", topStart), titleStyle)
		f.SetRowHeight(summarySheet, topStart, 28)

		topStart++
		f.SetCellValue(summarySheet, fmt.Sprintf("A%d", topStart), "#")
		f.SetCellValue(summarySheet, fmt.Sprintf("B%d", topStart), "Mahsulot")
		f.SetCellValue(summarySheet, fmt.Sprintf("C%d", topStart), "Miqdor (dona)")
		f.SetCellStyle(summarySheet, fmt.Sprintf("A%d", topStart), fmt.Sprintf("C%d", topStart), headerStyle)

		for i, tp := range rep.TopProducts {
			r := topStart + 1 + i
			f.SetCellValue(summarySheet, fmt.Sprintf("A%d", r), i+1)
			f.SetCellValue(summarySheet, fmt.Sprintf("B%d", r), tp.ProductName)
			f.SetCellValue(summarySheet, fmt.Sprintf("C%d", r), tp.TotalQty)
			st := dataStyle
			if i%2 == 1 {
				st = altRowStyle
			}
			f.SetCellStyle(summarySheet, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), st)
		}
	}

	// Column widths for summary
	f.SetColWidth(summarySheet, "A", "A", 28)
	f.SetColWidth(summarySheet, "B", "B", 28)
	f.SetColWidth(summarySheet, "C", "C", 18)

	// ── Sheet 2: Detailed Sales (Sotuvlar) ──────────────────────────────────
	salesSheet := "Sotuvlar"
	f.NewSheet(salesSheet)

	f.SetCellValue(salesSheet, "A1", fmt.Sprintf("%s — Batafsil sotuvlar", periodName))
	f.MergeCell(salesSheet, "A1", "Q1")
	f.SetCellStyle(salesSheet, "A1", "Q1", titleStyle)
	f.SetRowHeight(salesSheet, 1, 35)

	if salesErr != "" {
		// Show error on the sheet if query failed
		f.SetCellValue(salesSheet, "A3", "⚠️ "+salesErr)
		f.MergeCell(salesSheet, "A3", "H3")
	} else if len(sales) == 0 {
		f.SetCellValue(salesSheet, "A3", "Bu davrda sotuvlar amalga oshirilmagan.")
		f.MergeCell(salesSheet, "A3", "H3")
	} else {
		// Headers with full product details
		salesHeaders := []string{
			"#", "Sana", "Mahsulot", "Brend", "Davlat", "Kategoriya",
			"O'lcham", "Miqdor", "Tan narxi", "Sotish narxi",
			"Jami summa", "Foyda", "To'lov turi", "Holat",
			"Xaridor", "Telefon", "To'langan", "Qolgan qarz",
		}
		headerRow := 3
		for col, h := range salesHeaders {
			colLetter, _ := excelize.ColumnNumberToName(col + 1)
			f.SetCellValue(salesSheet, fmt.Sprintf("%s%d", colLetter, headerRow), h)
		}
		lastColLetter, _ := excelize.ColumnNumberToName(len(salesHeaders))
		f.SetCellStyle(salesSheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("%s%d", lastColLetter, headerRow), headerStyle)
		f.SetRowHeight(salesSheet, headerRow, 22)

		// Data rows
		for i, sale := range sales {
			r := headerRow + 1 + i
			col := func(n int) string {
				l, _ := excelize.ColumnNumberToName(n)
				return fmt.Sprintf("%s%d", l, r)
			}

			// Look up full product details
			var brand, country, category string
			costPrice := sale.CostPriceAtSaleSom
			if p, ok := productMap[sale.ProductID.Hex()]; ok {
				brand = p.Brand
				country = p.Country
				category = p.Category
				costPrice = p.CostPriceSom
			}

			totalAmount := sale.SellPriceAtSale * float64(sale.Quantity)
			unitProfit := sale.SellPriceAtSale - costPrice
			totalProfit := unitProfit * float64(sale.Quantity)

			f.SetCellValue(salesSheet, col(1), i+1)
			f.SetCellValue(salesSheet, col(2), sale.SoldAt.Format("02.01.2006 15:04"))
			f.SetCellValue(salesSheet, col(3), sale.ProductName)
			f.SetCellValue(salesSheet, col(4), brand)
			f.SetCellValue(salesSheet, col(5), country)
			f.SetCellValue(salesSheet, col(6), category)
			f.SetCellValue(salesSheet, col(7), sale.Size)
			f.SetCellValue(salesSheet, col(8), sale.Quantity)
			f.SetCellValue(salesSheet, col(9), services.FormatMoneyRaw(costPrice))
			f.SetCellValue(salesSheet, col(10), services.FormatMoneyRaw(sale.SellPriceAtSale))
			f.SetCellValue(salesSheet, col(11), services.FormatMoneyRaw(totalAmount))
			f.SetCellValue(salesSheet, col(12), services.FormatMoneyRaw(totalProfit))
			f.SetCellValue(salesSheet, col(13), paymentTypeUzbek(sale.PaymentType))
			f.SetCellValue(salesSheet, col(14), paymentStatusUzbek(sale.PaymentStatus))
			f.SetCellValue(salesSheet, col(15), sale.BuyerName)
			f.SetCellValue(salesSheet, col(16), sale.BuyerContact)
			f.SetCellValue(salesSheet, col(17), services.FormatMoneyRaw(sale.AmountPaid))
			f.SetCellValue(salesSheet, col(18), services.FormatMoneyRaw(sale.AmountDue))

			st := dataStyle
			if i%2 == 1 {
				st = altRowStyle
			}
			f.SetCellStyle(salesSheet, col(1), fmt.Sprintf("%s%d", lastColLetter, r), st)
		}

		// Column widths for sales sheet
		colWidths := map[string]float64{
			"A": 5, "B": 18, "C": 30, "D": 16, "E": 16, "F": 14,
			"G": 10, "H": 8, "I": 16, "J": 16,
			"K": 18, "L": 16, "M": 14, "N": 18,
			"O": 20, "P": 18, "Q": 18, "R": 18,
		}
		for col, w := range colWidths {
			f.SetColWidth(salesSheet, col, col, w)
		}
	}

	// ── Sheet 3: Product Catalog (Mahsulotlar) ──────────────────────────────
	catSheet := "Mahsulotlar"
	f.NewSheet(catSheet)

	f.SetCellValue(catSheet, "A1", "Mahsulotlar katalogi")
	f.MergeCell(catSheet, "A1", "L1")
	f.SetCellStyle(catSheet, "A1", "L1", titleStyle)
	f.SetRowHeight(catSheet, 1, 35)

	if productsErr != "" {
		f.SetCellValue(catSheet, "A3", "⚠️ "+productsErr)
		f.MergeCell(catSheet, "A3", "H3")
	} else if len(productMap) == 0 {
		f.SetCellValue(catSheet, "A3", "Mahsulotlar topilmadi.")
		f.MergeCell(catSheet, "A3", "H3")
	} else {
		catHeaders := []string{
			"#", "Mahsulot", "Brend", "Davlat", "Kategoriya",
			"Tan narxi", "Sotish narxi", "Foyda (1 dona)", "Holat",
			"O'lchamlar", "Jami zaxira", "Qo'shilgan sana",
		}
		headerRow := 3
		for col, h := range catHeaders {
			colLetter, _ := excelize.ColumnNumberToName(col + 1)
			f.SetCellValue(catSheet, fmt.Sprintf("%s%d", colLetter, headerRow), h)
		}
		lastColLetter, _ := excelize.ColumnNumberToName(len(catHeaders))
		f.SetCellStyle(catSheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("%s%d", lastColLetter, headerRow), headerStyle)
		f.SetRowHeight(catSheet, headerRow, 22)

		idx := 0
		for _, p := range productMap {
			r := headerRow + 1 + idx
			col := func(n int) string {
				l, _ := excelize.ColumnNumberToName(n)
				return fmt.Sprintf("%s%d", l, r)
			}

			// Build sizes string
			var sizeParts []string
			totalStock := 0
			for _, st := range p.Stock {
				totalStock += st.QuantityAvailable
				if st.Size != "" {
					sizeParts = append(sizeParts, fmt.Sprintf("%s:%d", st.Size, st.QuantityAvailable))
				}
			}
			sizesStr := strings.Join(sizeParts, ", ")
			if sizesStr == "" && totalStock > 0 {
				sizesStr = fmt.Sprintf("%d dona", totalStock)
			}

			statusLabel := "Faol"
			if p.Status == models.ProductStatusOutOfStock {
				statusLabel = "Tugagan"
			} else if p.Status == models.ProductStatusArchived {
				statusLabel = "Arxivlangan"
			}

			unitProfit := p.SellPrice - p.CostPriceSom

			f.SetCellValue(catSheet, col(1), idx+1)
			f.SetCellValue(catSheet, col(2), p.Name)
			f.SetCellValue(catSheet, col(3), p.Brand)
			f.SetCellValue(catSheet, col(4), p.Country)
			f.SetCellValue(catSheet, col(5), p.Category)
			f.SetCellValue(catSheet, col(6), services.FormatMoneyRaw(p.CostPriceSom))
			f.SetCellValue(catSheet, col(7), services.FormatMoneyRaw(p.SellPrice))
			f.SetCellValue(catSheet, col(8), services.FormatMoneyRaw(unitProfit))
			f.SetCellValue(catSheet, col(9), statusLabel)
			f.SetCellValue(catSheet, col(10), sizesStr)
			f.SetCellValue(catSheet, col(11), totalStock)
			f.SetCellValue(catSheet, col(12), p.CreatedAt.Format("02.01.2006"))

			st := dataStyle
			if idx%2 == 1 {
				st = altRowStyle
			}
			f.SetCellStyle(catSheet, col(1), fmt.Sprintf("%s%d", lastColLetter, r), st)
			idx++
		}

		// Column widths for catalog sheet
		catColWidths := map[string]float64{
			"A": 5, "B": 30, "C": 16, "D": 16, "E": 14,
			"F": 16, "G": 16, "H": 16, "I": 14,
			"J": 28, "K": 12, "L": 16,
		}
		for col, w := range catColWidths {
			f.SetColWidth(catSheet, col, col, w)
		}
	}

	// Save to temp file
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("report_%d.xlsx", time.Now().UnixMilli()))

	// Set "Sotuvlar" as active sheet so it opens there by default
	if salesSheetIdx, err := f.GetSheetIndex(salesSheet); err == nil {
		f.SetActiveSheet(salesSheetIdx)
	}

	if err := f.SaveAs(tmpPath); err != nil {
		return "", err
	}

	return tmpPath, nil
}

// calcTotalRevenue sums up total revenue from all sales in the period.
func calcTotalRevenue(sales []models.Sale) float64 {
	total := 0.0
	for _, s := range sales {
		total += s.SellPriceAtSale * float64(s.Quantity)
	}
	return total
}

// paymentTypeUzbek converts payment type constant to Uzbek label.
func paymentTypeUzbek(t string) string {
	switch t {
	case models.PaymentTypeCash:
		return "Naqd"
	case models.PaymentTypeCard:
		return "Karta"
	case models.PaymentTypeCredit:
		return "Nasiya"
	default:
		return t
	}
}

// paymentStatusUzbek converts payment status constant to Uzbek label.
func paymentStatusUzbek(s string) string {
	switch s {
	case models.PaymentStatusPaid:
		return "To'langan"
	case models.PaymentStatusPartiallyPaid:
		return "Qisman to'langan"
	case models.PaymentStatusUnpaid:
		return "To'lanmagan"
	default:
		return s
	}
}
