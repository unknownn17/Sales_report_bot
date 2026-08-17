package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"telegram-sales-bot/bot/state"
	"telegram-sales-bot/models"
	"telegram-sales-bot/services"

	ele "gopkg.in/telebot.v3"
)

func RegisterAddProduct(b *ele.Bot, app *AppContext) {
	// No callback buttons needed — Excel upload flow uses document handler
}

func startAddProduct(c ele.Context, app *AppContext) error {
	adminID := c.Sender().ID
	app.SessionMgr.ClearSession(adminID)
	app.SessionMgr.SetFlow(adminID, state.FlowAddProduct, state.AddStepWaitingExcel)
	return c.Send("📊 Mahsulotlar Excel faylini yuboring.\n\n" +
		"Excel faylida quyidagi ustunlar bo'lishi kerak:\n" +
		"• Mahsulot nomi (majburiy)\n" +
		"• Sotish narxi (majburiy)\n" +
		"• Tan narxi (majburiy)\n" +
		"• Brend, Davlat, Kategoriya, Miqdor, Razmerlar (ixtiyoriy)\n\n" +
		"Bekor qilish: /cancel")
}

// HandleAddProductDoc returns a handler that processes Excel document uploads during the add product flow.
func HandleAddProductDoc(app *AppContext) ele.HandlerFunc {
	return func(c ele.Context) error {
		adminID := c.Sender().ID
		if !app.AdminIDs[adminID] {
			return nil
		}

		sess := app.SessionMgr.GetSession(adminID)
		if sess.Flow != state.FlowAddProduct || sess.Step != state.AddStepWaitingExcel {
			return nil
		}

		doc := c.Message().Document
		if doc == nil || doc.FileID == "" {
			return c.Send("Iltimos, to'g'ri Excel fayl (.xlsx yoki .xls) yuboring.")
		}

		// Sanitize filename to prevent path traversal
		safeName := filepath.Base(doc.FileName)
		ext := strings.ToLower(filepath.Ext(safeName))
		if ext != ".xlsx" && ext != ".xls" {
			return c.Send("Iltimos, Excel formatidagi fayl (.xlsx yoki .xls) yuboring.")
		}

		// Download the file to a safe temp path
		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("products_%d_%d%s", adminID, time.Now().UnixMilli(), ext))
		if err := c.Bot().Download(&doc.File, tmpPath); err != nil {
			log.Printf("Failed to download Excel file: %v", err)
			return c.Send("❌ Faylni yuklab olishda xatolik. Qaytadan urinib ko'ring.")
		}
		defer os.Remove(tmpPath)

		// Parse Excel
		parser := services.NewExcelProductParser()
		parsedProducts, err := parser.ParseExcelFile(tmpPath)
		if err != nil {
			log.Printf("Excel parse error: %v", err)
			return c.Send(fmt.Sprintf("❌ Excel faylni o'qishda xatolik: %s", err.Error()))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var sb strings.Builder
		savedCount := 0
		errorCount := 0

		for i, pp := range parsedProducts {
			if len(pp.Errors) > 0 {
				errorCount++
				sb.WriteString(fmt.Sprintf("❌ Qator %d (%s): %s\n", i+2, pp.Product.Name, strings.Join(pp.Errors, "; ")))
				continue
			}

			pp.Product.AddedBy = adminID
			pp.Product.CreatedAt = time.Now()
			pp.Product.UpdatedAt = time.Now()

			// Upsert by product name: if an active product with the same name exists,
			// merge new stock into it and update other fields. Otherwise insert as new.
			existing, err := app.ProductRepo.FindProductByName(ctx, pp.Product.Name)
			if err != nil {
				errorCount++
				sb.WriteString(fmt.Sprintf("❌ Qator %d (%s): qidirishda xatolik\n", i+2, pp.Product.Name))
				continue
			}

			if existing != nil {
				// Update fields from the uploaded Excel row
				existing.Brand = pp.Product.Brand
				existing.Country = pp.Product.Country
				existing.Category = pp.Product.Category
				existing.SKU = pp.Product.SKU
				existing.SellPrice = pp.Product.SellPrice
				existing.CostPriceSom = pp.Product.CostPriceSom
				existing.Description = pp.Product.Description
				existing.Status = models.ProductStatusActive
				existing.UpdatedAt = time.Now()

				// Merge stock: add new arrivals to existing quantities
				existing.Stock = mergeStock(existing.Stock, pp.Product.Stock)
				existing.HasSizes = len(existing.Stock) > 0 && existing.Stock[0].Size != ""

				if err := app.ProductRepo.UpdateProduct(ctx, existing); err != nil {
					errorCount++
					sb.WriteString(fmt.Sprintf("❌ Qator %d (%s): yangilashda xatolik\n", i+2, pp.Product.Name))
					continue
				}
				savedCount++
				continue
			}

			// New product: keep it active regardless of initial stock quantity
			pp.Product.Status = models.ProductStatusActive

			if err := app.ProductRepo.InsertProduct(ctx, pp.Product); err != nil {
				errorCount++
				sb.WriteString(fmt.Sprintf("❌ Qator %d (%s): saqlashda xatolik\n", i+2, pp.Product.Name))
				continue
			}
			savedCount++
		}

		app.SessionMgr.ClearSession(adminID)

		sb.WriteString(fmt.Sprintf("\n📊 Natija:\n✅ %d mahsulot saqlandi", savedCount))
		if errorCount > 0 {
			sb.WriteString(fmt.Sprintf("\n❌ %d mahsulotda xatolik", errorCount))
		}

		c.Send(sb.String())
		return SendMainMenu(c)
	}
}

// mergeStock adds quantities from incoming stock items into existing ones by size.
// Sizes not yet present are appended. Empty-size items are treated as generic stock.
func mergeStock(existing, incoming []models.StockItem) []models.StockItem {
	merged := make([]models.StockItem, len(existing))
	copy(merged, existing)

	for _, in := range incoming {
		found := false
		for i := range merged {
			if merged[i].Size == in.Size {
				merged[i].QuantityAvailable += in.QuantityAvailable
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, in)
		}
	}

	return merged
}

// HandleAddProductPhoto returns a handler that responds when admin sends a photo instead of Excel.
func HandleAddProductPhoto(app *AppContext) ele.HandlerFunc {
	return func(c ele.Context) error {
		adminID := c.Sender().ID
		if !app.AdminIDs[adminID] {
			return nil
		}

		sess := app.SessionMgr.GetSession(adminID)
		if sess.Flow != state.FlowAddProduct || sess.Step != state.AddStepWaitingExcel {
			return nil
		}

		return c.Send("📊 Iltimos, rasm emas, Excel fayl (.xlsx) yuboring.\n\nBekor qilish: /cancel")
	}
}
