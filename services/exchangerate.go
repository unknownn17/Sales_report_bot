package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cbuAPIURLUSD = "https://cbu.uz/en/arkhiv-kursov-valyut/json/USD/"
	cbuAPIURLAll = "https://cbu.uz/en/arkhiv-kursov-valyut/json/"
	defaultFallbackRate = 12650.0
)

type ExchangeRateService struct {
	mu         sync.RWMutex
	cachedRate float64
	fetchedAt  time.Time
	httpClient *http.Client
}

type cbuCurrencyItem struct {
	Ccy     string `json:"Ccy"`
	CcyNmUZ string `json:"CcyNm_UZ"`
	Rate    string `json:"Rate"`
	Date    string `json:"Date"`
}

func NewExchangeRateService() *ExchangeRateService {
	return &ExchangeRateService{
		cachedRate: defaultFallbackRate,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchUSDRate returns the current USD/UZS exchange rate from CBU.
// It caches the rate in memory and falls back gracefully to cached/default if unreachable.
func (s *ExchangeRateService) FetchUSDRate(ctx context.Context) (float64, error) {
	s.mu.RLock()
	if s.cachedRate > 0 && time.Since(s.fetchedAt) < 1*time.Hour {
		rate := s.cachedRate
		s.mu.RUnlock()
		return rate, nil
	}
	s.mu.RUnlock()

	rate, err := s.fetchFromCBU(ctx, cbuAPIURLUSD)
	if err != nil {
		// Try fallback to all currencies endpoint
		rate, err = s.fetchFromCBU(ctx, cbuAPIURLAll)
	}

	if err != nil {
		s.mu.RLock()
		cached := s.cachedRate
		s.mu.RUnlock()
		if cached > 0 {
			return cached, nil
		}
		return defaultFallbackRate, fmt.Errorf("cbu api unavailable, using default: %w", err)
	}

	s.mu.Lock()
	s.cachedRate = rate
	s.fetchedAt = time.Now()
	s.mu.Unlock()

	return rate, nil
}

func (s *ExchangeRateService) fetchFromCBU(ctx context.Context, url string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "TelegramSalesBot/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http error: status %d", resp.StatusCode)
	}

	var items []cbuCurrencyItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, item := range items {
		if strings.EqualFold(item.Ccy, "USD") {
			r, err := strconv.ParseFloat(item.Rate, 64)
			if err == nil && r > 0 {
				return r, nil
			}
		}
	}

	return 0, fmt.Errorf("USD rate not found in response")
}
