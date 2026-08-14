package services

import (
	"testing"
)

func TestParseProductPost_Example(t *testing.T) {
	post := `🔥 TURKIYADAN YANGI KELDI! 🇹🇷
👕 SEAMLIFE brendidan ayol-qizlarimiz uchun chiroyli va qulay pijamalar! 🌸
✨ 100% TURKIYA MAHSULOTI 🇹🇷
💎 Sifatli
🌸 Qulay
✨ Uyda kiyishga juda mos
📏 Razmeri: XXXXL
💰 Narxi — atigi 240 000 so'm!`

	res := ParseProductPost(post)

	if !res.SellPriceFound || res.SellPrice != 240000 {
		t.Errorf("Expected price 240000, got %f (found: %v)", res.SellPrice, res.SellPriceFound)
	}

	if !res.BrandFound || res.Brand != "SEAMLIFE" {
		t.Errorf("Expected brand SEAMLIFE, got %s (found: %v)", res.Brand, res.BrandFound)
	}

	if !res.SizesFound || len(res.Sizes) != 1 || res.Sizes[0] != "XXXXL" {
		t.Errorf("Expected sizes [XXXXL], got %v (found: %v)", res.Sizes, res.SizesFound)
	}

	if !res.NameFound || res.Name == "" {
		t.Errorf("Expected name to be found, got %s", res.Name)
	}

	if !res.CountryFound || res.Country != "Turkiya 🇹🇷" {
		t.Errorf("Expected country 'Turkiya 🇹🇷', got '%s' (found: %v)", res.Country, res.CountryFound)
	}
}

func TestParseProductPost_MultiSize_China(t *testing.T) {
	post := `✨ Xitoydan keltirilgan yangi futbolkalar 🇨🇳
Brend: Nike
Razmer: S, M, L, XL
Narxi: 185.000 som`

	res := ParseProductPost(post)

	if !res.SellPriceFound || res.SellPrice != 185000 {
		t.Errorf("Expected price 185000, got %f", res.SellPrice)
	}

	if !res.SizesFound || len(res.Sizes) != 4 {
		t.Errorf("Expected 4 sizes, got %v", res.Sizes)
	}

	if !res.BrandFound || res.Brand != "NIKE" {
		t.Errorf("Expected brand NIKE, got %s", res.Brand)
	}

	if !res.CountryFound || res.Country != "Xitoy 🇨🇳" {
		t.Errorf("Expected country 'Xitoy 🇨🇳', got '%s'", res.Country)
	}
}
