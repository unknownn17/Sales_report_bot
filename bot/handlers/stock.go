package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	btnStockCountrySel  = (&ele.ReplyMarkup{}).Data("", "stk_cnt_sel")
	btnStockCategorySel = (&ele.ReplyMarkup{}).Data("", "stk_cat_sel")
	btnStockProductView = (&ele.ReplyMarkup{}).Data("", "stk_p_view")
	btnStockBackToList  = (&ele.ReplyMarkup{}).Data("🔙 Mahsulotlar ro'yxatiga qaytish", "stk_back_list")
	btnStockBackToCat   = (&ele.ReplyMarkup{}).Data("🔙 Kategoriyalarga qaytish", "stk_back_cat")
	btnStockBackToCnt   = (&ele.ReplyMarkup{}).Data("🔙 Davlatlarga qaytish", "stk_back_cnt")
)

func RegisterStock(b *ele.Bot, app *AppContext) {
	// 1. Country Selection Callback
	b.Handle(&btnStockCountrySel, func(c ele.Context) error {
		country := c.Callback().Data
		adminID := c.Sender().ID
		app.SessionMgr.SetData(adminID, "stk_country", country)
		return renderStockCategoryMenu(c, app, country, true)
	})

	// 2. Category Selection Callback
	b.Handle(&btnStockCategorySel, func(c ele.Context) error {
		category := c.Callback().Data
		adminID := c.Sender().ID
		app.SessionMgr.SetData(adminID, "stk_category", category)
		country := app.SessionMgr.GetDataString(adminID, "stk_country")
		return renderStockProductList(c, app, country, category, true)
	})

	// 3. Product Number Button Callback
	b.Handle(&btnStockProductView, func(c ele.Context) error {
		idHex := c.Callback().Data
		pid, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			return nil
		}
		_ = c.Delete()
		return renderProductDetail(c, app, pid)
	})

	// 4. Back to Product List Callback
	b.Handle(&btnStockBackToList, func(c ele.Context) error {
		adminID := c.Sender().ID
		country := app.SessionMgr.GetDataString(adminID, "stk_country")
		category := app.SessionMgr.GetDataString(adminID, "stk_category")
		_ = c.Delete()
		return renderStockProductList(c, app, country, category, false)
	})

	// 5. Back to Categories Callback
	b.Handle(&btnStockBackToCat, func(c ele.Context) error {
		adminID := c.Sender().ID
		country := app.SessionMgr.GetDataString(adminID, "stk_country")
		_ = c.Delete()
		return renderStockCategoryMenu(c, app, country, false)
	})

	// 6. Back to Countries Callback
	b.Handle(&btnStockBackToCnt, func(c ele.Context) error {
		_ = c.Delete()
		return renderStockCountryMenu(c, app, false)
	})
}

func startStockHierarchy(c ele.Context, app *AppContext) error {
	return renderStockCountryMenu(c, app, false)
}

// Step 1: Render Country Selection Menu
func renderStockCountryMenu(c ele.Context, app *AppContext, isEdit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	countries, err := app.ProductRepo.FindDistinctCountries(ctx)
	if err != nil {
		return c.Send("❌ Ma'lumotlarni yuklashda xatolik.")
	}

	if len(countries) == 0 {
		return c.Send("📦 Omborda faol mahsulotlar mavjud emas.")
	}

	menu := &ele.ReplyMarkup{}
	var rows []ele.Row
	var currentRow []ele.Btn

	for _, cnt := range countries {
		btn := menu.Data(cnt, "stk_cnt_sel", cnt)
		currentRow = append(currentRow, btn)
		if len(currentRow) == 2 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	rows = append(rows, menu.Row(menu.Data("🌐 Barcha davlatlar", "stk_cnt_sel", "all")))
	menu.Inline(rows...)

	msg := "📦 Ombor / Zaxira\n\nQaysi davlat mahsulotlarini ko'rmoqchisiz?"
	if isEdit && c.Callback() != nil {
		return c.Edit(msg, menu)
	}
	return c.Send(msg, menu)
}

// Step 2: Render Category Selection Menu for Selected Country
func renderStockCategoryMenu(c ele.Context, app *AppContext, country string, isEdit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filterCountry := country
	if country == "all" {
		filterCountry = ""
	}

	categories, err := app.ProductRepo.FindDistinctCategoriesByCountry(ctx, filterCountry)
	if err != nil {
		return c.Send("❌ Kategoriyalarni yuklashda xatolik.")
	}

	countryDisplay := "Barcha davlatlar"
	if filterCountry != "" {
		countryDisplay = filterCountry
	}

	if len(categories) == 0 {
		return c.Send(fmt.Sprintf("'%s' bo'yicha hech qanday mahsulot topilmadi.", countryDisplay))
	}

	menu := &ele.ReplyMarkup{}
	var rows []ele.Row
	var currentRow []ele.Btn

	for _, cat := range categories {
		btn := menu.Data(fmt.Sprintf("📁 %s", cat), "stk_cat_sel", cat)
		currentRow = append(currentRow, btn)
		if len(currentRow) == 2 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	rows = append(rows, menu.Row(menu.Data("📂 Barcha kategoriyalar", "stk_cat_sel", "all")))
	rows = append(rows, menu.Row(btnStockBackToCnt))
	menu.Inline(rows...)

	msg := fmt.Sprintf("🌐 Davlat: %s\n\nKategoriyani tanlang:", countryDisplay)
	if isEdit && c.Callback() != nil {
		return c.Edit(msg, menu)
	}
	return c.Send(msg, menu)
}

// Step 3: Render Numbered Products List with Inline Number Buttons
func renderStockProductList(c ele.Context, app *AppContext, country, category string, isEdit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filterCountry := country
	if country == "all" {
		filterCountry = ""
	}
	filterCategory := category
	if category == "all" {
		filterCategory = ""
	}

	products, err := app.ProductRepo.FindProductsByCountryAndCategory(ctx, filterCountry, filterCategory)
	if err != nil {
		return c.Send("❌ Mahsulotlarni yuklashda xatolik.")
	}

	if len(products) == 0 {
		return c.Send("Ushbu filtr bo'yicha mahsulotlar topilmadi.")
	}

	var sb strings.Builder
	countryLabel := "Barcha davlatlar"
	if filterCountry != "" {
		countryLabel = filterCountry
	}
	categoryLabel := "Barcha kategoriyalar"
	if filterCategory != "" {
		categoryLabel = filterCategory
	}

	sb.WriteString(fmt.Sprintf("📦 Ombor holati (%s | %s):\n\n", countryLabel, categoryLabel))

	menu := &ele.ReplyMarkup{}
	var numRow []ele.Btn
	var rows []ele.Row

	for i, p := range products {
		brandTag := ""
		if p.Brand != "" {
			brandTag = fmt.Sprintf("[%s] ", p.Brand)
		}

		sb.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, brandTag, p.Name))
		sb.WriteString(fmt.Sprintf("   💰 Narxi: %s\n", services.FormatMoney(p.SellPrice)))

		var sizeParts []string
		totalQty := 0
		for _, st := range p.Stock {
			totalQty += st.QuantityAvailable
			flag := ""
			if st.QuantityAvailable <= 0 {
				flag = " 🔴"
			} else if st.QuantityAvailable <= app.LowStockThreshold {
				flag = fmt.Sprintf(" ⚠️ (Kam: %d)", st.QuantityAvailable)
			}

			if st.Size != "" {
				sizeParts = append(sizeParts, fmt.Sprintf("%s: %d ta%s", st.Size, st.QuantityAvailable, flag))
			}
		}

		if len(sizeParts) > 0 {
			sb.WriteString(fmt.Sprintf("   📏 O'lchamlar: %s\n\n", strings.Join(sizeParts, " | ")))
		} else {
			flag := ""
			if totalQty <= 0 {
				flag = " 🔴 (Tugagan)"
			} else if totalQty <= app.LowStockThreshold {
				flag = fmt.Sprintf(" ⚠️ (Kam: %d)", totalQty)
			}
			sb.WriteString(fmt.Sprintf("   🔢 Miqdor: %d dona%s\n\n", totalQty, flag))
		}

		// Create number button matching the item order
		btn := menu.Data(fmt.Sprintf("%d", i+1), "stk_p_view", p.ID.Hex())
		numRow = append(numRow, btn)

		// Group buttons in rows of 5
		if len(numRow) == 5 || i == len(products)-1 {
			rows = append(rows, menu.Row(numRow...))
			numRow = nil
		}
	}

	rows = append(rows, menu.Row(btnStockBackToCat))
	menu.Inline(rows...)

	if isEdit && c.Callback() != nil {
		return c.Edit(sb.String(), menu)
	}
	return c.Send(sb.String(), menu)
}

// Step 4: Render Full Product Detail Modal with Photo
func renderProductDetail(c ele.Context, app *AppContext, productID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := app.ProductRepo.FindProductByID(ctx, productID)
	if err != nil || p == nil {
		return c.Send("❌ Mahsulot topilmadi.")
	}

	unitProfit := p.SellPrice - p.CostPriceSom

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🛍 <b>%s</b>\n\n", p.Name))
	if p.Brand != "" {
		sb.WriteString(fmt.Sprintf("🏢 <b>Brend:</b> %s\n", p.Brand))
	}
	if p.Country != "" {
		sb.WriteString(fmt.Sprintf("🌐 <b>Keltirilgan davlat:</b> %s\n", p.Country))
	}
	if p.Category != "" {
		sb.WriteString(fmt.Sprintf("📂 <b>Kategoriya:</b> %s\n", p.Category))
	}
	sb.WriteString(fmt.Sprintf("💰 <b>Sotish narxi:</b> %s\n", services.FormatMoney(p.SellPrice)))
	sb.WriteString(fmt.Sprintf("💳 <b>Tan narxi:</b> %s\n", services.FormatMoney(p.CostPriceSom)))
	sb.WriteString(fmt.Sprintf("📈 <b>Foyda (1 dona):</b> %s\n\n", services.FormatMoney(unitProfit)))

	sb.WriteString("📦 <b>Zaxira (Ombor):</b>\n")
	totalStock := 0
	for _, st := range p.Stock {
		totalStock += st.QuantityAvailable
		flag := ""
		if st.QuantityAvailable <= 0 {
			flag = " 🔴 (Tugagan)"
		} else if st.QuantityAvailable <= app.LowStockThreshold {
			flag = fmt.Sprintf(" ⚠️ (Kam qolgan: %d ta)", st.QuantityAvailable)
		}

		if st.Size != "" {
			sb.WriteString(fmt.Sprintf("  • <b>%s:</b> %d dona%s\n", st.Size, st.QuantityAvailable, flag))
		} else {
			sb.WriteString(fmt.Sprintf("  • <b>Jami:</b> %d dona%s\n", st.QuantityAvailable, flag))
		}
	}
	sb.WriteString(fmt.Sprintf("📊 <b>Jami mavjud:</b> %d dona\n", totalStock))

	if p.Description != "" {
		sb.WriteString(fmt.Sprintf("\n📝 <b>Tavsif (Post matni):</b>\n<i>%s</i>\n", p.Description))
	}

	menu := &ele.ReplyMarkup{}
	menu.Inline(
		menu.Row(btnStockBackToList),
	)

	// If photo is available, send with photo!
	if p.PhotoFileID != "" {
		photo := &ele.Photo{
			File:    ele.File{FileID: p.PhotoFileID},
			Caption: sb.String(),
		}
		return c.Send(photo, menu, ele.ModeHTML)
	}

	return c.Send(sb.String(), menu, ele.ModeHTML)
}
