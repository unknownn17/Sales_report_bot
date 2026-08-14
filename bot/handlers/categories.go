package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"telegram-sales-bot/bot/state"

	ele "gopkg.in/telebot.v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	btnCatAdd    = (&ele.ReplyMarkup{}).Data("➕ Yangi kategoriya qo'shish", "cat_add")
	btnCatDelete = (&ele.ReplyMarkup{}).Data("", "cat_del")
)

func RegisterCategories(b *ele.Bot, app *AppContext) {
	// Add category button callback
	b.Handle(&btnCatAdd, func(c ele.Context) error {
		adminID := c.Sender().ID
		app.SessionMgr.SetFlow(adminID, state.FlowCategoryManage, state.CategoryStepEnterName)
		_ = c.Delete()
		return c.Send("Yangi kategoriya nomini kiriting (masalan: Kostyum, Jinsi, Ko'ylak):\nBekor qilish: /cancel")
	})

	// Delete category callback
	b.Handle(&btnCatDelete, func(c ele.Context) error {
		idHex := c.Callback().Data
		catID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = app.CategoryRepo.DeleteCategory(ctx, catID)
		_ = c.Respond(&ele.CallbackResponse{Text: "Kategoriya o'chirildi", ShowAlert: false})
		return renderCategoriesMenu(c, app, true)
	})
}

func showCategoriesMenu(c ele.Context, app *AppContext) error {
	return renderCategoriesMenu(c, app, false)
}

func renderCategoriesMenu(c ele.Context, app *AppContext, isEdit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cats, err := app.CategoryRepo.GetAllCategories(ctx)
	if err != nil {
		return c.Send("❌ Kategoriyalarni yuklashda xatolik.")
	}

	var sb strings.Builder
	sb.WriteString("📁 Mavjud kategoriyalar:\n\n")

	menu := &ele.ReplyMarkup{}
	var rows []ele.Row

	if len(cats) == 0 {
		sb.WriteString("Hozircha hech qanday kategoriya yo'q.\n")
	} else {
		for i, cat := range cats {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cat.Name))
			// Delete button
			delBtn := menu.Data(fmt.Sprintf("🗑 %s", cat.Name), "cat_del", cat.ID.Hex())
			rows = append(rows, menu.Row(delBtn))
		}
	}

	rows = append(rows, menu.Row(btnCatAdd))
	menu.Inline(rows...)

	if isEdit && c.Callback() != nil {
		return c.Edit(sb.String(), menu)
	}
	return c.Send(sb.String(), menu)
}

func handleCategoryManageText(c ele.Context, app *AppContext, sess *state.Session) error {
	adminID := c.Sender().ID
	text := strings.TrimSpace(c.Message().Text)

	if sess.Step == state.CategoryStepEnterName {
		if text == "" || text == "/skip" {
			return c.Send("Kategoriya nomini kiriting:")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cat, err := app.CategoryRepo.CreateCategory(ctx, text, adminID)
		if err != nil {
			return c.Send(fmt.Sprintf("❌ '%s' nomli kategoriya allaqachon mavjud yoki xatolik yuz berdi.", text))
		}

		app.SessionMgr.ClearSession(adminID)
		c.Send(fmt.Sprintf("✅ '%s' kategoriyasi muvaffaqiyatli qo'shildi!", cat.Name))
		return SendMainMenu(c)
	}

	return nil
}
