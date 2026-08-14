package services

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type ParseResult struct {
	Name           string
	Brand          string
	Country        string
	SellPrice      float64
	Sizes          []string
	NameFound      bool
	BrandFound     bool
	CountryFound   bool
	SellPriceFound bool
	SizesFound     bool
}

var (
	priceRegex = regexp.MustCompile(`(?i)(?:narx[i]?|baho[si]?)\s*[:—\-–]*\s*(?:atigi\s+)?([0-9\s,.'’]+)`)
	sizeRegex  = regexp.MustCompile(`(?i)(?:razmer[ilar]*|o['’]?lcham[lar]*|razmeri)\s*[:—\-–]*\s*([^\n\r]+)`)
	brandRegex = regexp.MustCompile(`(?i)([A-Z0-9&'\-]{2,})\s+brend[a-z]*|brend[a-z]*\s*[:—\-–]?\s*([A-Z0-9&'\-]{2,})`)
	emojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}\x{2600}-\x{27BF}\x{FE00}-\x{FE0F}\x{1F900}-\x{1F9FF}\x{1FA00}-\x{1FAFF}]`)

	knownStandardSizes = map[string]bool{
		"XXS": true, "XS": true, "S": true, "M": true, "L": true,
		"XL": true, "2XL": true, "XXL": true, "3XL": true, "XXXL": true,
		"4XL": true, "XXXXL": true, "5XL": true, "XXXXXL": true, "6XL": true,
		"36": true, "38": true, "40": true, "42": true, "44": true, "46": true,
		"48": true, "50": true, "52": true, "54": true, "56": true, "58": true, "60": true,
	}

	knownBrands = []string{
		"SEAMLIFE", "LC WAIKIKI", "WAIKIKI", "DEFACTO", "KOTON", "ZARA",
		"MANGO", "BERSHKA", "PULL&BEAR", "PUMA", "NIKE", "ADIDAS",
		"TERRA PRO", "COLLIN'S", "MAVI", "U.S. POLO", "POLO", "GUCCI",
		"AYDOG'AN", "AYDOGAN", "AYDOGʻAN",
	}

	clothingKeywords = []string{
		"pijama", "pijamalar", "futbolka", "futbolkalar", "shim", "shimlar",
		"ko'ylak", "koylak", "koʻylak", "kurtka", "sviter", "hudi", "hoodie",
		"kostyum", "shortik", "tolstovka", "jinsi", "xalat", "komplekt", "kiyim",
	}
)

type countryMapping struct {
	keywords []string
	name     string
}

var countryMappings = []countryMapping{
	{keywords: []string{"TURKIYA", "TURKIYADAN", "TURKEY", "TURK", "🇹🇷"}, name: "Turkiya 🇹🇷"},
	{keywords: []string{"XITOY", "XITOYDAN", "CHINA", "🇨🇳"}, name: "Xitoy 🇨🇳"},
	{keywords: []string{"DUBAY", "DUBAI", "BAA", "UAE", "🇦🇪"}, name: "BAA (Dubay) 🇦🇪"},
	{keywords: []string{"O'ZBEKISTON", "OZBEKISTON", "UZBEKISTAN", "UZB", "🇺🇿"}, name: "O'zbekiston 🇺🇿"},
	{keywords: []string{"QIRG'IZISTON", "QIRGIZISTON", "BISHKEK", "KYRGYZSTAN", "🇰🇬"}, name: "Qirg'iziston 🇰🇬"},
	{keywords: []string{"VYETNAM", "VIETNAM", "🇻🇳"}, name: "Vyetnam 🇻🇳"},
	{keywords: []string{"KOREYA", "KOREA", "🇰🇷"}, name: "Koreya 🇰🇷"},
	{keywords: []string{"ITALIYA", "ITALY", "🇮🇹"}, name: "Italiya 🇮🇹"},
}

func ParseProductPost(text string) *ParseResult {
	res := &ParseResult{}
	rawLines := strings.Split(text, "\n")

	// Clean lines for analysis
	var cleanLines []string
	for _, l := range rawLines {
		cleaned := strings.TrimSpace(emojiRegex.ReplaceAllString(l, ""))
		if cleaned != "" {
			cleanLines = append(cleanLines, cleaned)
		}
	}

	// 1. Extract Price
	if match := priceRegex.FindStringSubmatch(text); len(match) > 1 {
		rawNum := match[1]
		cleanNum := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, rawNum)

		if val, err := strconv.ParseFloat(cleanNum, 64); err == nil && val > 0 {
			res.SellPrice = val
			res.SellPriceFound = true
		}
	}

	// 2. Extract Sizes
	if match := sizeRegex.FindStringSubmatch(text); len(match) > 1 {
		rawSizeLine := match[1]
		tokens := strings.FieldsFunc(rawSizeLine, func(r rune) bool {
			return r == ',' || r == '/' || r == '-' || r == '–' || r == '—' || r == '|' || unicode.IsSpace(r)
		})

		seen := make(map[string]bool)
		for _, tok := range tokens {
			norm := strings.ToUpper(strings.Trim(tok, " .,:;!?()[]{}'\""))
			if norm == "" {
				continue
			}
			if knownStandardSizes[norm] || isNumericSize(norm) {
				if !seen[norm] {
					seen[norm] = true
					res.Sizes = append(res.Sizes, norm)
				}
			}
		}
		if len(res.Sizes) > 0 {
			res.SizesFound = true
		}
	}

	// 3. Extract Country of Origin
	textUpper := strings.ToUpper(text)
	for _, cm := range countryMappings {
		for _, kw := range cm.keywords {
			if strings.Contains(textUpper, kw) || strings.Contains(text, kw) {
				res.Country = cm.name
				res.CountryFound = true
				break
			}
		}
		if res.CountryFound {
			break
		}
	}

	// 4. Extract Brand
	for _, kb := range knownBrands {
		if strings.Contains(textUpper, kb) {
			res.Brand = kb
			res.BrandFound = true
			break
		}
	}

	if !res.BrandFound {
		if match := brandRegex.FindStringSubmatch(text); len(match) > 0 {
			candidate := match[1]
			if candidate == "" && len(match) > 2 {
				candidate = match[2]
			}
			candidate = strings.ToUpper(strings.Trim(candidate, " .,:;!?'\""))
			if len(candidate) >= 2 && candidate != "TURKIYA" && candidate != "YANGI" {
				res.Brand = candidate
				res.BrandFound = true
			}
		}
	}

	if !res.BrandFound {
		for _, line := range cleanLines {
			words := strings.Fields(line)
			for _, w := range words {
				cw := strings.ToUpper(strings.Trim(w, " .,:;!?()[]{}'\""))
				if len(cw) >= 3 && isAllLetters(cw) && cw != "TURKIYA" && cw != "YANGI" && cw != "KELDI" && cw != "MAHSULOTI" && cw != "SIFATLI" && cw != "QULAY" {
					res.Brand = cw
					res.BrandFound = true
					break
				}
			}
			if res.BrandFound {
				break
			}
		}
	}

	// 5. Extract Name / Description Line
	for _, line := range cleanLines {
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "narx") || strings.HasPrefix(lowerLine, "razmer") {
			continue
		}

		for _, kw := range clothingKeywords {
			if strings.Contains(lowerLine, kw) {
				res.Name = line
				res.NameFound = true
				break
			}
		}
		if res.NameFound {
			break
		}
	}

	if !res.NameFound {
		for _, line := range cleanLines {
			lowerLine := strings.ToLower(line)
			if !strings.HasPrefix(lowerLine, "narx") && !strings.HasPrefix(lowerLine, "razmer") && len(line) >= 4 {
				res.Name = line
				res.NameFound = true
				break
			}
		}
	}

	return res
}

func isNumericSize(s string) bool {
	if n, err := strconv.Atoi(s); err == nil && n >= 20 && n <= 70 {
		return true
	}
	return false
}

func isAllLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && r != '&' && r != '-' {
			return false
		}
	}
	return true
}
