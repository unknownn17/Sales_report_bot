package services

import (
	"testing"
	"time"
)

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0 so'm"},
		{1250000, "1 250 000 so'm"},
		{240000, "240 000 so'm"},
		{500, "500 so'm"},
		{12650, "12 650 so'm"},
		{-50000, "-50 000 so'm"},
	}

	for _, tt := range tests {
		got := FormatMoney(tt.input)
		if got != tt.expected {
			t.Errorf("FormatMoney(%f) = %s, expected %s", tt.input, got, tt.expected)
		}
	}
}

func TestCalculateUnitProfit(t *testing.T) {
	sellPrice := 240000.0
	costSom := 150000.0
	profit := CalculateUnitProfit(sellPrice, costSom)
	if profit != 90000.0 {
		t.Errorf("Expected 90000.0, got %f", profit)
	}
}

func TestDateRanges(t *testing.T) {
	todayStart, todayEnd := TodayRange()
	if !todayEnd.After(todayStart) {
		t.Errorf("TodayRange end must be after start")
	}
	if todayEnd.Sub(todayStart) != 24*time.Hour {
		t.Errorf("TodayRange should span 24 hours")
	}

	weekStart, weekEnd := ThisWeekRange()
	if !weekEnd.After(weekStart) {
		t.Errorf("ThisWeekRange end must be after start")
	}

	monthStart, monthEnd := ThisMonthRange()
	if !monthEnd.After(monthStart) {
		t.Errorf("ThisMonthRange end must be after start")
	}
}
