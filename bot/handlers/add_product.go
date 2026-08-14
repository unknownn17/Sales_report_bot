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
)

var (
	btnAPCountrySelect = (&ele.ReplyMarkup{}).Data("", "ap_country_sel")
	btnAPCatSelect     = (&ele.ReplyMarkup{}).Data("", "ap_cat_select")
	btnAPHasSizesYes   = (&ele.ReplyMarkup{}).Data("✅ Ha, bor", "ap_has_sizes_yes")
	btnAPHasSizesNo    = (&ele.ReplyMarkup{}).Data("❌ Yo'q (Bitta o'lcham)", "ap_has_sizes_no")
	btnAPSave          = (&ele.ReplyMarkup{}).Data("✅ Saqlash", "ap_save")
	btnAPEditField     = (&ele.ReplyMarkup{}).Data("", "ap_edit")
)

func RegisterAddProduct(b *ele.Bot, app *AppContext) {
	// Country selection callback
	b.Handle(&btnAPCountrySelect, func(c ele.Context) error {
		adminID := c.Sender().ID
		country := c.Callback().Data

		if country == "custom" {
			app.SessionMgr.SetStep(adminID, state.AddStepCustomCountry)
			_ = c.Delete()
			return c.Send("✏️ Keltirilgan davlat nomini kiriting:")
		}

		app.SessionMgr.SetData(adminID, "country", country)
		_ = c.Delete()
		c.Send(fmt.Sprintf("✅ Keltirilgan davlat: %s", country))
		return promptNextMissingOrCost(c, app)
	})

	// Category selection callback
	b.Handle(&btnAPCatSelect, func(c ele.Context) error {
		adminID := c.Sender().ID
		cat := c.Callback().Data

		if cat == "add_new" {
			app.SessionMgr.SetStep(adminID, state.AddStepAskCategory)
			_ = c.Delete()
			return c.Send("Yangi kategoriya nomini yozing:")
		}

		app.SessionMgr.SetData(adminID, "category", cat)
		_ = c.Delete()
		c.Send(fmt.Sprintf("✅ Kategoriya: %s", cat))
		return promptNextMissingOrCost(c, app)
	})

	// Has sizes callback
	b.Handle(&btnAPHasSizesYes, func(c ele.Context) error {
		adminID := c.Sender().ID
		app.SessionMgr.SetData(adminID, "has_sizes", true)
		_ = c.Delete()

		sizes := app.SessionMgr.GetSizes(adminID)
		if len(sizes) > 0 {
			app.SessionMgr.SetData(adminID, "size_idx", 0)
			app.SessionMgr.SetStep(adminID, state.AddStepAskSizeQty)
			return c.Send(fmt.Sprintf("📏 '%s' o'lcham uchun nechta borligini kiriting:", sizes[0]))
		}

		app.SessionMgr.SetStep(adminID, state.AddStepAskHasSizes)
		return c.Send("O'lchamlarni vergul bilan kiriting (masalan: S, M, L, XL yoki 42, 44, 46):")
	})

	b.Handle(&btnAPHasSizesNo, func(c ele.Context) error {
		adminID := c.Sender().ID
		app.SessionMgr.SetData(adminID, "has_sizes", false)
		app.SessionMgr.SetData(adminID, "sizes", []string{})
		app.SessionMgr.SetStep(adminID, state.AddStepAskSingleQty)
		_ = c.Delete()
		return c.Send("Umumiy nechta dona borligini kiriting:")
	})

	// Edit field button callback
	b.Handle(&btnAPEditField, func(c ele.Context) error {
		adminID := c.Sender().ID
		field := c.Callback().Data
		app.SessionMgr.SetData(adminID, "edit_field", field)
		app.SessionMgr.SetStep(adminID, state.AddStepEditValue)
		_ = c.Delete()

		var fieldName string
		switch field {
		case "name":
			fieldName = "Mahsulot nomini"
		case "brand":
			fieldName = "Brend nomini"
		case "country":
			fieldName = "Keltirilgan davlatni"
		case "sell_price":
			fieldName = "Sotish narxini (so'mda)"
		case "cost_som":
			fieldName = "Tan narxini (so'mda)"
		default:
			fieldName = "Yangi qiymatni"
		}
		return c.Send(fmt.Sprintf("✏️ %s kiriting:", fieldName))
	})

	// Save product callback
	b.Handle(&btnAPSave, func(c ele.Context) error {
		_ = c.Delete()
		return saveProduct(c, app)
	})
}

func startAddProduct(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	app.SessionMgr.ClearSession(adminID)
	app.SessionMgr.SetFlow(adminID, state.FlowAddProduct, state.AddStepWaitingPost)
	return c.Send("Mahsulot postini yuboring (yoki rasmli xabar sifatida yo'llang):\n\nBekor qilish uchun: /cancel")
}

func handleAddProductText(c ele.Context, app *AppContext, sess *state.Session) error {
	adminID := c.Sender().ID
	text := strings.TrimSpace(c.Message().Text)

	// Step 1: Waiting for post
	if sess.Step == state.AddStepWaitingPost {
		caption := c.Message().Caption
		postContent := text
		if postContent == "" && caption != "" {
			postContent = caption
		}

		if c.Message().Photo != nil {
			app.SessionMgr.SetData(adminID, "photo_file_id", c.Message().Photo.FileID)
		}

		app.SessionMgr.SetData(adminID, "description", postContent)
		parsed := services.ParseProductPost(postContent)

		if parsed.SellPriceFound {
			app.SessionMgr.SetData(adminID, "sell_price", parsed.SellPrice)
		}
		if parsed.BrandFound {
			app.SessionMgr.SetData(adminID, "brand", parsed.Brand)
		}
		if parsed.CountryFound {
			app.SessionMgr.SetData(adminID, "country", parsed.Country)
		}
		if parsed.NameFound {
			app.SessionMgr.SetData(adminID, "name", parsed.Name)
		}
		if parsed.SizesFound && len(parsed.Sizes) > 0 {
			app.SessionMgr.SetData(adminID, "sizes", parsed.Sizes)
			app.SessionMgr.SetData(adminID, "has_sizes", true)
		}

		// Detected fields summary
		var sb strings.Builder
		sb.WriteString("Aniqlandi:\n")

		if parsed.CountryFound {
			sb.WriteString(fmt.Sprintf("✅ Davlat: %s\n", parsed.Country))
		} else {
			sb.WriteString("❌ Davlat — aniqlanmadi\n")
		}

		if parsed.BrandFound {
			sb.WriteString(fmt.Sprintf("✅ Brend: %s\n", parsed.Brand))
		} else {
			sb.WriteString("❌ Brend — aniqlanmadi\n")
		}

		if parsed.SellPriceFound {
			sb.WriteString(fmt.Sprintf("✅ Narxi: %s\n", services.FormatMoney(parsed.SellPrice)))
		} else {
			sb.WriteString("❌ Narxi — aniqlanmadi\n")
		}

		if parsed.SizesFound && len(parsed.Sizes) > 0 {
			sb.WriteString(fmt.Sprintf("✅ Razmerlar: %s\n", strings.Join(parsed.Sizes, ", ")))
		} else {
			sb.WriteString("❌ Razmerlar — aniqlanmadi\n")
		}

		if parsed.NameFound {
			sb.WriteString(fmt.Sprintf("✅ Nomi: %s\n", parsed.Name))
		} else {
			sb.WriteString("❌ Nomi — aniqlanmadi\n")
		}

		c.Send(sb.String())
		return promptNextMissingOrCost(c, app)
	}

	// Step: Custom Country
	if sess.Step == state.AddStepCustomCountry {
		if text != "" {
			app.SessionMgr.SetData(adminID, "country", text)
		}
		return promptNextMissingOrCost(c, app)
	}

	// Step: Prompting missing name
	if sess.Step == state.AddStepAskName {
		if text != "/skip" && text != "" {
			app.SessionMgr.SetData(adminID, "name", text)
		}
		return promptNextMissingOrCost(c, app)
	}

	// Step: Create category on the fly
	if sess.Step == state.AddStepAskCategory {
		if text != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = app.CategoryRepo.CreateCategory(ctx, text, adminID)
			app.SessionMgr.SetData(adminID, "category", text)
		}
		return promptNextMissingOrCost(c, app)
	}

	// Step: Prompting Cost Price in Som
	if sess.Step == state.AddStepAskCostSom {
		cleaned := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, text)

		costSom, err := strconv.ParseFloat(cleaned, 64)
		if err != nil || costSom <= 0 {
			return c.Send("Iltimos, to'g'ri son kiriting (masalan: 140 000):")
		}
		app.SessionMgr.SetData(adminID, "cost_price_som", costSom)

		sellPrice := app.SessionMgr.GetDataFloat(adminID, "sell_price")
		unitProfit := sellPrice - costSom

		msg := fmt.Sprintf("Tan narxi: %s\nFoyda (1 dona uchun): %s\n\n",
			services.FormatMoney(costSom), services.FormatMoney(unitProfit))

		sizes := app.SessionMgr.GetSizes(adminID)
		if len(sizes) > 0 {
			app.SessionMgr.SetData(adminID, "has_sizes", true)
			app.SessionMgr.SetData(adminID, "size_idx", 0)
			app.SessionMgr.SetStep(adminID, state.AddStepAskSizeQty)
			c.Send(msg)
			return c.Send(fmt.Sprintf("📏 '%s' o'lcham uchun nechta borligini kiriting:", sizes[0]))
		}

		app.SessionMgr.SetStep(adminID, state.AddStepAskHasSizes)
		menu := &ele.ReplyMarkup{}
		menu.Inline(menu.Row(btnAPHasSizesYes, btnAPHasSizesNo))
		c.Send(msg)
		return c.Send("Mahsulotda o'lchamlar (razmerlar) bormi?", menu)
	}

	// Step: Asking custom sizes list
	if sess.Step == state.AddStepAskHasSizes {
		rawTokens := strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == '/' || r == '-' || r == ' '
		})
		var cleanSizes []string
		for _, tok := range rawTokens {
			tok = strings.ToUpper(strings.TrimSpace(tok))
			if tok != "" {
				cleanSizes = append(cleanSizes, tok)
			}
		}
		if len(cleanSizes) == 0 {
			return c.Send("Kamida bitta o'lcham kiriting (masalan: M, L, XL):")
		}

		app.SessionMgr.SetData(adminID, "sizes", cleanSizes)
		app.SessionMgr.SetData(adminID, "has_sizes", true)
		app.SessionMgr.SetData(adminID, "size_idx", 0)
		app.SessionMgr.SetStep(adminID, state.AddStepAskSizeQty)
		return c.Send(fmt.Sprintf("📏 '%s' o'lcham uchun nechta borligini kiriting:", cleanSizes[0]))
	}

	// Step: Asking quantity per size
	if sess.Step == state.AddStepAskSizeQty {
		qty, err := strconv.Atoi(text)
		if err != nil || qty < 0 {
			return c.Send("Miqdorni butun son sifatida kiriting:")
		}

		sizes := app.SessionMgr.GetSizes(adminID)
		sizeIdx := app.SessionMgr.GetDataInt(adminID, "size_idx")

		var currentItems []map[string]interface{}
		if existing := app.SessionMgr.GetStockItems(adminID); existing != nil {
			currentItems = existing
		}

		currentItems = append(currentItems, map[string]interface{}{
			"Size":              sizes[sizeIdx],
			"QuantityAvailable": qty,
		})
		app.SessionMgr.SetData(adminID, "stock_items", currentItems)

		if sizeIdx+1 < len(sizes) {
			app.SessionMgr.SetData(adminID, "size_idx", sizeIdx+1)
			return c.Send(fmt.Sprintf("📏 '%s' o'lcham uchun nechta borligini kiriting:", sizes[sizeIdx+1]))
		}

		return showProductSummary(c, app)
	}

	// Step: Asking single quantity for no-size product
	if sess.Step == state.AddStepAskSingleQty {
		qty, err := strconv.Atoi(text)
		if err != nil || qty < 0 {
			return c.Send("Miqdorni butun son sifatida kiriting:")
		}

		items := []map[string]interface{}{
			{
				"Size":              "",
				"QuantityAvailable": qty,
			},
		}
		app.SessionMgr.SetData(adminID, "stock_items", items)
		return showProductSummary(c, app)
	}

	// Step: Edit a specific field value
	if sess.Step == state.AddStepEditValue {
		field := app.SessionMgr.GetDataString(adminID, "edit_field")
		val := text
		switch field {
		case "name":
			app.SessionMgr.SetData(adminID, "name", val)
		case "brand":
			app.SessionMgr.SetData(adminID, "brand", strings.ToUpper(val))
		case "country":
			app.SessionMgr.SetData(adminID, "country", val)
		case "sell_price":
			cleaned := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, val)
			if p, err := strconv.ParseFloat(cleaned, 64); err == nil && p > 0 {
				app.SessionMgr.SetData(adminID, "sell_price", p)
			}
		case "cost_som":
			cleaned := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, val)
			if p, err := strconv.ParseFloat(cleaned, 64); err == nil && p > 0 {
				app.SessionMgr.SetData(adminID, "cost_price_som", p)
			}
		}
		return showProductSummary(c, app)
	}

	return nil
}

func promptNextMissingOrCost(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID

	// 1. Check Country
	country := app.SessionMgr.GetDataString(adminID, "country")
	if country == "" {
		app.SessionMgr.SetStep(adminID, state.AddStepAskCountry)
		menu := &ele.ReplyMarkup{}
		var rows []ele.Row
		var currentRow []ele.Btn
		for _, cnt := range models.DefaultCountries {
			btn := menu.Data(cnt, "ap_country_sel", cnt)
			currentRow = append(currentRow, btn)
			if len(currentRow) == 2 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = nil
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		rows = append(rows, menu.Row(menu.Data("✏️ Boshqa davlat", "ap_country_sel", "custom")))
		menu.Inline(rows...)
		return c.Send("Mahsulot qaysi davlatdan keltirilgan?", menu)
	}

	// 2. Check Name
	name := app.SessionMgr.GetDataString(adminID, "name")
	if name == "" {
		app.SessionMgr.SetStep(adminID, state.AddStepAskName)
		return c.Send("❌ Mahsulot nomini kiriting (yoki /skip):")
	}

	// 3. Check Category
	cat := app.SessionMgr.GetDataString(adminID, "category")
	if cat == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cats, _ := app.CategoryRepo.GetAllCategories(ctx)

		menu := &ele.ReplyMarkup{}
		var rows []ele.Row
		var currentRow []ele.Btn
		for _, category := range cats {
			btn := menu.Data(category.Name, "ap_cat_select", category.Name)
			currentRow = append(currentRow, btn)
			if len(currentRow) == 2 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = nil
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		rows = append(rows, menu.Row(menu.Data("➕ Yangi kategoriya kiritish", "ap_cat_select", "add_new")))
		menu.Inline(rows...)
		return c.Send("Mahsulot kategoriyasini tanlang:", menu)
	}

	// 4. Check Sell Price
	sellPrice := app.SessionMgr.GetDataFloat(adminID, "sell_price")
	if sellPrice <= 0 {
		app.SessionMgr.SetData(adminID, "edit_field", "sell_price")
		app.SessionMgr.SetStep(adminID, state.AddStepEditValue)
		return c.Send("Sotish narxini kiriting (masalan: 240 000):")
	}

	// 5. Prompt Cost Price in Som directly
	app.SessionMgr.SetStep(adminID, state.AddStepAskCostSom)
	return c.Send("Mahsulotning tan narxini so'mda kiriting (masalan: 140 000):")
}

func showProductSummary(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	app.SessionMgr.SetStep(adminID, state.AddStepShowSummary)

	name := app.SessionMgr.GetDataString(adminID, "name")
	brand := app.SessionMgr.GetDataString(adminID, "brand")
	country := app.SessionMgr.GetDataString(adminID, "country")
	category := app.SessionMgr.GetDataString(adminID, "category")
	sellPrice := app.SessionMgr.GetDataFloat(adminID, "sell_price")
	costSom := app.SessionMgr.GetDataFloat(adminID, "cost_price_som")
	unitProfit := sellPrice - costSom

	var sb strings.Builder
	sb.WriteString("📋 Mahsulot ma'lumotlari:\n\n")
	sb.WriteString(fmt.Sprintf("🏷 Nomi: %s\n", name))
	if country != "" {
		sb.WriteString(fmt.Sprintf("🌐 Davlat: %s\n", country))
	}
	if brand != "" {
		sb.WriteString(fmt.Sprintf("🏢 Brend: %s\n", brand))
	}
	if category != "" {
		sb.WriteString(fmt.Sprintf("📂 Kategoriya: %s\n", category))
	}
	sb.WriteString(fmt.Sprintf("💰 Sotish narxi: %s\n", services.FormatMoney(sellPrice)))
	sb.WriteString(fmt.Sprintf("💳 Tan narxi: %s\n", services.FormatMoney(costSom)))
	sb.WriteString(fmt.Sprintf("📈 Kutilayotgan foyda (1 dona): %s\n\n", services.FormatMoney(unitProfit)))

	items := app.SessionMgr.GetStockItems(adminID)
	sb.WriteString("📦 Zaxira (Ombor):\n")
	totalCount := 0
	for _, it := range items {
		size, _ := it["Size"].(string)
		qty, _ := it["QuantityAvailable"].(int)
		totalCount += qty
		if size != "" {
			sb.WriteString(fmt.Sprintf("  • %s: %d dona\n", size, qty))
		} else {
			sb.WriteString(fmt.Sprintf("  • Jami: %d dona\n", qty))
		}
	}
	sb.WriteString(fmt.Sprintf("Jami miqdor: %d dona\n", totalCount))

	menu := &ele.ReplyMarkup{}
	menu.Inline(
		menu.Row(
			menu.Data("✏️ Nomi", "ap_edit", "name"),
			menu.Data("✏️ Davlat", "ap_edit", "country"),
		),
		menu.Row(
			menu.Data("✏️ Brend", "ap_edit", "brand"),
			menu.Data("✏️ Narxi", "ap_edit", "sell_price"),
		),
		menu.Row(
			menu.Data("✏️ Tan narxi", "ap_edit", "cost_som"),
		),
		menu.Row(btnAPSave),
	)

	return c.Send(sb.String(), menu)
}

func saveProduct(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	items := app.SessionMgr.GetStockItems(adminID)

	var stock []models.StockItem
	totalCount := 0
	for _, it := range items {
		size, _ := it["Size"].(string)
		qty, _ := it["QuantityAvailable"].(int)
		totalCount += qty
		stock = append(stock, models.StockItem{
			Size:              size,
			QuantityAvailable: qty,
		})
	}

	costSom := app.SessionMgr.GetDataFloat(adminID, "cost_price_som")
	status := models.ProductStatusActive
	if totalCount == 0 {
		status = models.ProductStatusOutOfStock
	}

	prod := &models.Product{
		Name:         app.SessionMgr.GetDataString(adminID, "name"),
		Brand:        app.SessionMgr.GetDataString(adminID, "brand"),
		Country:      app.SessionMgr.GetDataString(adminID, "country"),
		Category:     app.SessionMgr.GetDataString(adminID, "category"),
		HasSizes:     app.SessionMgr.GetDataBool(adminID, "has_sizes"),
		CostPriceSom: costSom,
		SellPrice:    app.SessionMgr.GetDataFloat(adminID, "sell_price"),
		Description:  app.SessionMgr.GetDataString(adminID, "description"),
		PhotoFileID:  app.SessionMgr.GetDataString(adminID, "photo_file_id"),
		Status:       status,
		Stock:        stock,
		AddedBy:      adminID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ProductRepo.InsertProduct(ctx, prod); err != nil {
		log.Printf("InsertProduct error: %v", err)
		return c.Send("❌ Mahsulotni saqlashda xatolik yuz berdi. Qaytadan urinib ko'ring.")
	}

	app.SessionMgr.ClearSession(adminID)
	c.Send("✅ Mahsulot muvaffaqiyatli saqlandi!")
	return SendMainMenu(c)
}
