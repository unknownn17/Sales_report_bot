package bot

import ele "gopkg.in/telebot.v3"

// Menu button labels in Uzbek
const (
	BtnAddProduct   = "➕ Mahsulot qo'shish"
	BtnConfirmSale  = "💰 Sotuvni tasdiqlash"
	BtnStock        = "📦 Ombor / Zaxira"
	BtnCategories   = "📁 Kategoriyalar"
	BtnTodayReport  = "📊 Bugungi hisobot"
	BtnPeriodReport = "📈 Haftalik/Oylik hisobot"
	BtnDebtors      = "📋 Qarzdorlar"
	BtnDueSoon      = "🔔 Muddati yaqin"
)

// MainMenu creates the reply keyboard for the admin
func MainMenu() *ele.ReplyMarkup {
	menu := &ele.ReplyMarkup{ResizeKeyboard: true}

	menu.Reply(
		menu.Row(menu.Text(BtnAddProduct), menu.Text(BtnConfirmSale)),
		menu.Row(menu.Text(BtnStock), menu.Text(BtnCategories)),
		menu.Row(menu.Text(BtnTodayReport), menu.Text(BtnPeriodReport)),
		menu.Row(menu.Text(BtnDebtors), menu.Text(BtnDueSoon)),
	)

	return menu
}

// SendMainMenu sends the main menu reply keyboard
func SendMainMenu(c ele.Context) error {
	return c.Send("Asosiy menyu:", MainMenu())
}
