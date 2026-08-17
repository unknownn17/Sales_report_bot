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

	"go.mongodb.org/mongo-driver/bson/primitive"
	ele "gopkg.in/telebot.v3"
)

var (
	btnCSProductSelect = (&ele.ReplyMarkup{}).Data("", "cs_p_sel")
	btnCSProductPage   = (&ele.ReplyMarkup{}).Data("", "cs_p_page")
	btnCSSizeSelect    = (&ele.ReplyMarkup{}).Data("", "cs_s_sel")
	btnCSPaymentType   = (&ele.ReplyMarkup{}).Data("", "cs_ptype")
	btnCSConfirmSale   = (&ele.ReplyMarkup{}).Data("✅ Tasdiqlash", "cs_confirm_sale")
	btnCSCancelSale    = (&ele.ReplyMarkup{}).Data("❌ Bekor qilish", "cs_cancel_sale")
)

func RegisterConfirmSale(b *ele.Bot, app *AppContext) {
	// Product selection callback
	b.Handle(&btnCSProductSelect, func(c ele.Context) error {
		idHex := c.Callback().Data
		pid, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			return nil
		}

		adminID := c.Sender().ID
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		prod, err := app.ProductRepo.FindProductByID(ctx, pid)
		if err != nil || prod == nil {
			return c.Send("❌ Mahsulot topilmadi.")
		}

		app.SessionMgr.SetData(adminID, "sale_product_id", prod.ID)
		app.SessionMgr.SetData(adminID, "sale_product_name", prod.Name)
		app.SessionMgr.SetData(adminID, "sale_sell_price", prod.SellPrice)
		app.SessionMgr.SetData(adminID, "sale_cost_som", prod.CostPriceSom)
		app.SessionMgr.SetData(adminID, "sale_has_sizes", prod.HasSizes)

		// Check stock availability
		if !prod.HasSizes || len(prod.Stock) == 0 {
			available := 0
			if len(prod.Stock) > 0 {
				available = prod.Stock[0].QuantityAvailable
			}
			if available <= 0 {
				return c.Send("❌ Ushbu mahsulot omborda qolmagan.")
			}
			app.SessionMgr.SetData(adminID, "sale_size", "")
			app.SessionMgr.SetStep(adminID, state.SaleStepAskQuantity)
			return c.Edit(fmt.Sprintf("Mahsulot: %s\nMavjud: %d dona\n\nSotilgan miqdorni kiriting:", prod.Name, available))
		}

		// Has sizes -> show sizes with live counts
		menu := &ele.ReplyMarkup{}
		var rows []ele.Row
		var currentRow []ele.Btn
		hasAvailableSize := false

		for _, st := range prod.Stock {
			btnText := fmt.Sprintf("%s (%d dona)", st.Size, st.QuantityAvailable)
			if st.QuantityAvailable <= 0 {
				btnText = fmt.Sprintf("❌ %s (0 ta)", st.Size)
				// Size is out of stock - clickable button that gives feedback
				btn := menu.Data(btnText, "cs_s_sel", fmt.Sprintf("0:%s", st.Size))
				currentRow = append(currentRow, btn)
			} else {
				hasAvailableSize = true
				btn := menu.Data(btnText, "cs_s_sel", st.Size)
				currentRow = append(currentRow, btn)
			}

			if len(currentRow) == 2 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = nil
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}

		if !hasAvailableSize {
			return c.Edit(fmt.Sprintf("❌ '%s' mahsulotining barcha o'lchamlari tugagan.", prod.Name))
		}

		menu.Inline(rows...)
		app.SessionMgr.SetStep(adminID, state.SaleStepSelectSize)
		return c.Edit(fmt.Sprintf("Mahsulot: %s\nO'lchamni tanlang:", prod.Name), menu)
	})

	// Product list pagination callback
	b.Handle(&btnCSProductPage, func(c ele.Context) error {
		pageStr := c.Callback().Data
		page, _ := strconv.Atoi(pageStr)
		if page <= 0 {
			page = 1
		}
		return renderProductList(c, app, page, app.SessionMgr.GetDataString(c.Sender().ID, "sale_search_query"))
	})

	// Size select callback
	b.Handle(&btnCSSizeSelect, func(c ele.Context) error {
		adminID := c.Sender().ID
		sizeData := c.Callback().Data

		if strings.HasPrefix(sizeData, "0:") {
			return c.Respond(&ele.CallbackResponse{
				Text:      "Bu o'lchamda mahsulot qolmagan!",
				ShowAlert: true,
			})
		}

		app.SessionMgr.SetData(adminID, "sale_size", sizeData)
		app.SessionMgr.SetStep(adminID, state.SaleStepAskQuantity)

		pname := app.SessionMgr.GetDataString(adminID, "sale_product_name")
		return c.Edit(fmt.Sprintf("Mahsulot: %s\nO'lcham: %s\n\nSotilgan miqdorni kiriting:", pname, sizeData))
	})

	// Payment type callback
	b.Handle(&btnCSPaymentType, func(c ele.Context) error {
		adminID := c.Sender().ID
		ptype := c.Callback().Data
		app.SessionMgr.SetData(adminID, "payment_type", ptype)

		qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")
		sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
		total := sellPrice * float64(qty)

		if ptype == models.PaymentTypeCredit {
			app.SessionMgr.SetStep(adminID, state.SaleStepAskBuyerName)
			return c.Edit(fmt.Sprintf("To'lov turi: 📝 Nasiya (Qarz)\nJami summa: %s\n\n👤 Xaridor ismini kiriting (majburiy):", services.FormatMoney(total)))
		}

		// Cash or Card
		app.SessionMgr.SetData(adminID, "amount_paid", total)
		app.SessionMgr.SetData(adminID, "amount_due", 0.0)
		app.SessionMgr.SetStep(adminID, state.SaleStepAskBuyerName)

		typeName := "💵 Naqd"
		if ptype == models.PaymentTypeCard {
			typeName = "💳 Karta"
		}
		return c.Edit(fmt.Sprintf("To'lov turi: %s\nJami summa: %s\n\n👤 Xaridor ismini kiriting (yoki /skip):", typeName, services.FormatMoney(total)))
	})

	// Confirm sale button
	b.Handle(&btnCSConfirmSale, func(c ele.Context) error {
		return finalizeSale(c, app)
	})

	// Cancel sale button
	b.Handle(&btnCSCancelSale, func(c ele.Context) error {
		return HandleCancel(c, app)
	})
}

func startConfirmSale(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	app.SessionMgr.ClearSession(adminID)
	app.SessionMgr.SetFlow(adminID, state.FlowConfirmSale, state.SaleStepSelectProduct)
	return renderProductList(c, app, 1, "")
}

func renderProductList(c ele.Context, app *AppContext, page int, search string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pageSize := 6
	products, total, err := app.ProductRepo.FindActiveProducts(ctx, search, page, pageSize)
	if err != nil {
		return c.Send("❌ Mahsulotlar ro'yxatini yuklashda xatolik.")
	}

	if len(products) == 0 {
		if search != "" {
			return c.Send(fmt.Sprintf("❌ '%s' bo'yicha mahsulot topilmadi.\nBoshqa so'z bilan qidirib ko'ring yoki /cancel bosing.", search))
		}
		return c.Send("Omborda faol mahsulotlar mavjud emas.")
	}

	menu := &ele.ReplyMarkup{}
	var rows []ele.Row

	for _, p := range products {
		btnText := fmt.Sprintf("%s — %s", p.Name, services.FormatMoney(p.SellPrice))
		btn := menu.Data(btnText, "cs_p_sel", p.ID.Hex())
		rows = append(rows, menu.Row(btn))
	}

	// Pagination row
	var navBtns []ele.Btn
	if page > 1 {
		navBtns = append(navBtns, menu.Data("◀️ Oldingi", "cs_p_page", strconv.Itoa(page-1)))
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages > 1 {
		navBtns = append(navBtns, menu.Data(fmt.Sprintf("%d/%d", page, totalPages), "cs_p_page", strconv.Itoa(page)))
	}
	if int64(page*pageSize) < total {
		navBtns = append(navBtns, menu.Data("Keyingi ▶️", "cs_p_page", strconv.Itoa(page+1)))
	}
	if len(navBtns) > 0 {
		rows = append(rows, menu.Row(navBtns...))
	}

	menu.Inline(rows...)
	header := "💰 Sotilgan mahsulotni tanlang:\n(Qidirish uchun mahsulot yoki brend nomini yozib yuborishingiz mumkin)"
	if search != "" {
		header = fmt.Sprintf("🔍 Qidiruv: '%s' (Jami %d ta):\nMahsulotni tanlang:", search, total)
	}

	if c.Callback() != nil {
		return c.Edit(header, menu)
	}
	return c.Send(header, menu)
}

func handleConfirmSaleText(c ele.Context, app *AppContext, sess *state.Session) error {
	adminID := c.Sender().ID
	text := strings.TrimSpace(c.Message().Text)

	// In product selection step, if text is entered, treat as search query
	if sess.Step == state.SaleStepSelectProduct {
		app.SessionMgr.SetData(adminID, "sale_search_query", text)
		return renderProductList(c, app, 1, text)
	}

	// Quantity input step
	if sess.Step == state.SaleStepAskQuantity {
		qty, err := strconv.Atoi(text)
		if err != nil || qty <= 0 {
			return c.Send("Iltimos, musbat butun son kiriting (masalan: 1 yoki 2):")
		}

		pid := app.SessionMgr.GetDataObjectID(adminID, "sale_product_id")
		size := app.SessionMgr.GetDataString(adminID, "sale_size")

		// Attempt atomic stock decrement
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = app.ProductRepo.DecrementStock(ctx, pid, size, qty)
		if err != nil {
			log.Printf("Stock decrement failed: %v", err)
			return c.Send(fmt.Sprintf("❌ Zaxirada yetarli miqdor yo'q! Kamroq miqdor kiriting yoki /cancel bosing:"))
		}

		// Stock successfully reserved
		app.SessionMgr.SetData(adminID, "sale_qty", qty)
		app.SessionMgr.SetData(adminID, "stock_reserved", true)

		sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
		total := sellPrice * float64(qty)

		menu := &ele.ReplyMarkup{}
		menu.Inline(
			menu.Row(
				menu.Data("💵 Naqd", "cs_ptype", models.PaymentTypeCash),
				menu.Data("💳 Karta", "cs_ptype", models.PaymentTypeCard),
			),
			menu.Row(
				menu.Data("📝 Nasiya (Qarz)", "cs_ptype", models.PaymentTypeCredit),
			),
		)

		app.SessionMgr.SetStep(adminID, state.SaleStepAskPaymentType)
		return c.Send(fmt.Sprintf("✅ %d dona zaxiradan band qilindi.\nJami summa: %s\n\nTo'lov turini tanlang:", qty, services.FormatMoney(total)), menu)
	}

	// Buyer Name step
	if sess.Step == state.SaleStepAskBuyerName {
		ptype := app.SessionMgr.GetDataString(adminID, "payment_type")
		if text != "/skip" && text != "" {
			app.SessionMgr.SetData(adminID, "buyer_name", text)
		} else {
			if ptype == models.PaymentTypeCredit {
				return c.Send("❌ Nasiya (qarz) uchun xaridor ismi majburiy! Ismni kiriting:")
			}
			app.SessionMgr.SetData(adminID, "buyer_name", "Noma'lum xaridor")
		}

		if ptype == models.PaymentTypeCredit {
			app.SessionMgr.SetStep(adminID, state.SaleStepAskBuyerContact)
			return c.Send("📞 Xaridor telefon raqami yoki kontaktini kiriting (yoki /skip):")
		}

		// For Cash/Card, proceed to confirmation
		return showSaleConfirmation(c, app)
	}

	// Buyer Contact step (for credit)
	if sess.Step == state.SaleStepAskBuyerContact {
		if text != "/skip" && text != "" {
			app.SessionMgr.SetData(adminID, "buyer_contact", text)
		}
		app.SessionMgr.SetStep(adminID, state.SaleStepAskAmountPaid)

		qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")
		sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
		total := sellPrice * float64(qty)

		return c.Send(fmt.Sprintf("Jami summa: %s\n\nHozir to'langan summani kiriting (agar hech narsa to'lanmagan bo'lsa 0 deb yozing):", services.FormatMoney(total)))
	}

	// Amount Paid step (for credit)
	if sess.Step == state.SaleStepAskAmountPaid {
		cleaned := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, text)
		if cleaned == "" {
			cleaned = "0"
		}
		paid, err := strconv.ParseFloat(cleaned, 64)
		if err != nil || paid < 0 {
			return c.Send("Iltimos, to'g'ri summa kiriting:")
		}

		qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")
		sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
		total := sellPrice * float64(qty)

		if paid > total {
			return c.Send(fmt.Sprintf("To'langan summa jami summadan (%s) katta bo'lishi mumkin emas. Qaytadan kiriting:", services.FormatMoney(total)))
		}

		app.SessionMgr.SetData(adminID, "amount_paid", paid)
		app.SessionMgr.SetData(adminID, "amount_due", total-paid)
		app.SessionMgr.SetStep(adminID, state.SaleStepAskDueDate)

		return c.Send("📅 Qarz qaytarish muddatini kiriting (masalan: 15 kun yoki 25.08.2026, yoki /skip):")
	}

	// Due Date step (for credit)
	if sess.Step == state.SaleStepAskDueDate {
		if text != "/skip" && text != "" {
			// Try parsing as number of days
			if days, err := strconv.Atoi(text); err == nil && days > 0 {
				dueDate := time.Now().AddDate(0, 0, days)
				app.SessionMgr.SetData(adminID, "due_date", dueDate)
			} else if parsedTime, err := time.Parse("02.01.2006", text); err == nil {
				app.SessionMgr.SetData(adminID, "due_date", parsedTime)
			} else if parsedTime, err := time.Parse("2006-01-02", text); err == nil {
				app.SessionMgr.SetData(adminID, "due_date", parsedTime)
			}
		}
		return showSaleConfirmation(c, app)
	}

	return nil
}

func showSaleConfirmation(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	app.SessionMgr.SetStep(adminID, state.SaleStepConfirm)

	pname := app.SessionMgr.GetDataString(adminID, "sale_product_name")
	size := app.SessionMgr.GetDataString(adminID, "sale_size")
	qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")
	sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
	costSom := app.SessionMgr.GetDataFloat(adminID, "sale_cost_som")
	ptype := app.SessionMgr.GetDataString(adminID, "payment_type")
	paid := app.SessionMgr.GetDataFloat(adminID, "amount_paid")
	due := app.SessionMgr.GetDataFloat(adminID, "amount_due")
	buyer := app.SessionMgr.GetDataString(adminID, "buyer_name")
	contact := app.SessionMgr.GetDataString(adminID, "buyer_contact")

	totalAmount := sellPrice * float64(qty)
	unitProfit := sellPrice - costSom
	totalProfit := unitProfit * float64(qty)

	var typeLabel string
	switch ptype {
	case models.PaymentTypeCash:
		typeLabel = "💵 Naqd"
	case models.PaymentTypeCard:
		typeLabel = "💳 Karta"
	case models.PaymentTypeCredit:
		typeLabel = "📝 Nasiya (Qarz)"
	}

	var sb strings.Builder
	sb.WriteString("📋 Sotuvni tasdiqlash:\n\n")
	sb.WriteString(fmt.Sprintf("🛍 Mahsulot: %s\n", pname))
	if size != "" {
		sb.WriteString(fmt.Sprintf("📏 O'lcham: %s\n", size))
	}
	sb.WriteString(fmt.Sprintf("🔢 Miqdor: %d dona\n", qty))
	sb.WriteString(fmt.Sprintf("💰 Narxi (1 dona): %s\n", services.FormatMoney(sellPrice)))
	sb.WriteString(fmt.Sprintf("💵 Jami summa: %s\n", services.FormatMoney(totalAmount)))
	sb.WriteString(fmt.Sprintf("💳 To'lov turi: %s\n", typeLabel))
	sb.WriteString(fmt.Sprintf("✅ To'langan: %s\n", services.FormatMoney(paid)))
	if due > 0 {
		sb.WriteString(fmt.Sprintf("⏳ Qolgan qarz: %s\n", services.FormatMoney(due)))
	}
	if buyer != "" {
		sb.WriteString(fmt.Sprintf("👤 Xaridor: %s\n", buyer))
	}
	if contact != "" {
		sb.WriteString(fmt.Sprintf("📞 Telefon: %s\n", contact))
	}
	if dueDateVal, ok := app.SessionMgr.GetData(adminID, "due_date"); ok && dueDateVal != nil {
		if dt, ok := dueDateVal.(time.Time); ok {
			sb.WriteString(fmt.Sprintf("📅 To'lov muddati: %s\n", services.FormatDateShort(dt)))
		}
	}
	sb.WriteString(fmt.Sprintf("\n📈 Kutilayotgan foyda: %s (1 donaga %s)\n", services.FormatMoney(totalProfit), services.FormatMoney(unitProfit)))

	menu := &ele.ReplyMarkup{}
	menu.Inline(
		menu.Row(btnCSConfirmSale, btnCSCancelSale),
	)

	return c.Send(sb.String(), menu)
}

func finalizeSale(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID

	pid := app.SessionMgr.GetDataObjectID(adminID, "sale_product_id")
	pname := app.SessionMgr.GetDataString(adminID, "sale_product_name")
	size := app.SessionMgr.GetDataString(adminID, "sale_size")
	qty := app.SessionMgr.GetDataInt(adminID, "sale_qty")
	sellPrice := app.SessionMgr.GetDataFloat(adminID, "sale_sell_price")
	costSom := app.SessionMgr.GetDataFloat(adminID, "sale_cost_som")
	ptype := app.SessionMgr.GetDataString(adminID, "payment_type")
	paid := app.SessionMgr.GetDataFloat(adminID, "amount_paid")
	due := app.SessionMgr.GetDataFloat(adminID, "amount_due")
	buyer := app.SessionMgr.GetDataString(adminID, "buyer_name")
	contact := app.SessionMgr.GetDataString(adminID, "buyer_contact")

	var status string
	if due <= 0 {
		status = models.PaymentStatusPaid
		paid = sellPrice * float64(qty)
		due = 0
	} else if paid > 0 {
		status = models.PaymentStatusPartiallyPaid
	} else {
		status = models.PaymentStatusUnpaid
	}

	var dueDate *time.Time
	if dueDateVal, ok := app.SessionMgr.GetData(adminID, "due_date"); ok && dueDateVal != nil {
		if dt, ok := dueDateVal.(time.Time); ok {
			dueDate = &dt
		}
	}

	sale := &models.Sale{
		ProductID:          pid,
		ProductName:        pname,
		Size:               size,
		Quantity:           qty,
		SellPriceAtSale:    sellPrice,
		CostPriceAtSaleSom: costSom,
		PaymentType:        ptype,
		PaymentStatus:      status,
		AmountPaid:         paid,
		AmountDue:          due,
		DueDate:            dueDate,
		BuyerName:          buyer,
		BuyerContact:       contact,
		SoldAt:             time.Now(),
		SoldBy:             adminID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.SaleRepo.InsertSale(ctx, sale); err != nil {
		log.Printf("InsertSale error: %v", err)
		return c.Send("❌ Sotuvni saqlashda xatolik yuz berdi.")
	}

	// If initial payment made, record in payments collection
	if paid > 0 {
		payment := &models.Payment{
			SaleID: sale.ID,
			Amount: paid,
			PaidAt: time.Now(),
			Note:   "Boshlang'ich to'lov",
		}
		if err := app.PaymentRepo.InsertPayment(ctx, payment); err != nil {
			log.Printf("InsertPayment error: %v", err)
		}
	}

	// Remove product entirely if stock is fully depleted
	_ = app.ProductRepo.CheckAndUpdateStockStatus(ctx, pid)

	// Clear session without rollback since sale completed
	app.SessionMgr.SetData(adminID, "stock_reserved", false)
	app.SessionMgr.ClearSession(adminID)

	unitProfit := sellPrice - costSom
	totalProfit := unitProfit * float64(qty)

	c.Send(fmt.Sprintf("✅ Sotuv muvaffaqiyatli qayd etildi!\n💰 Foyda: %s", services.FormatMoney(totalProfit)))
	return SendMainMenu(c)
}
