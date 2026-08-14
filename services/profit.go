package services

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// FormatMoney formats a float64 as Uzbek som with space thousands separators.
// Example: 1250000 -> "1 250 000 so'm"
func FormatMoney(amount float64) string {
	return FormatMoneyRaw(amount) + " so'm"
}

// FormatMoneyRaw formats without the "so'm" suffix.
func FormatMoneyRaw(amount float64) string {
	rounded := int64(math.Round(amount))
	isNegative := rounded < 0
	if isNegative {
		rounded = -rounded
	}

	str := fmt.Sprintf("%d", rounded)
	n := len(str)
	var parts []string

	for i := n; i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{str[start:i]}, parts...)
	}

	result := strings.Join(parts, " ")
	if isNegative {
		result = "-" + result
	}
	return result
}

// CalculateUnitProfit returns sellPrice - costPriceSom.
func CalculateUnitProfit(sellPrice, costPriceSom float64) float64 {
	return sellPrice - costPriceSom
}

// TodayRange returns the start and end of today in local time.
func TodayRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	return start, end
}

// ThisWeekRange returns start of Monday 00:00:00 and end of Sunday 23:59:59 (next Monday 00:00:00).
func ThisWeekRange() (time.Time, time.Time) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday in Go is 0, we treat Monday as 1
		weekday = 7
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start = start.AddDate(0, 0, -(weekday - 1))
	end := start.AddDate(0, 0, 7)
	return start, end
}

// ThisMonthRange returns start and end of current month.
func ThisMonthRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	return start, end
}

var uzbekMonths = []string{
	"", "Yanvar", "Fevral", "Mart", "Aprel", "May", "Iyun",
	"Iyul", "Avgust", "Sentyabr", "Oktyabr", "Noyabr", "Dekabr",
}

// FormatDateLong formats time as "14 Avgust 2026"
func FormatDateLong(t time.Time) string {
	m := int(t.Month())
	mStr := ""
	if m >= 1 && m <= 12 {
		mStr = uzbekMonths[m]
	}
	return fmt.Sprintf("%d %s %d", t.Day(), mStr, t.Year())
}

// FormatDateShort formats time as "14.08.2026"
func FormatDateShort(t time.Time) string {
	return t.Format("02.01.2006")
}
