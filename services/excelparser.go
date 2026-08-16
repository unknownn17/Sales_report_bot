package services

import (
	"fmt"
	"strconv"
	"strings"

	"telegram-sales-bot/models"

	"github.com/xuri/excelize/v2"
)

// ParsedProduct represents a parsed product from Excel with flexibility
type ParsedProduct struct {
	Product    *models.Product
	Errors     []string
	Warnings   []string
	ExtraCells map[string]interface{} // Unknown/extra columns from Excel
}

// ExcelProductParser handles parsing product data from Excel files
type ExcelProductParser struct {
	requiredFields []string
	fieldMappings  map[string][]string // Maps field to possible column headers
}

// NewExcelProductParser creates a new parser with flexible field mapping
func NewExcelProductParser() *ExcelProductParser {
	return &ExcelProductParser{
		requiredFields: []string{"name", "sell_price", "cost_price_som"},
		fieldMappings: map[string][]string{
			"name":           {"name", "product_name", "mahsulot", "mahsulot nomi", "title", "heading"},
			"brand":          {"brand", "brend", "brendlar"},
			"country":        {"country", "davlat", "qaysi davlatdan", "keltirilgan davlat", "origin", "import_country"},
			"category":       {"category", "kategoriya", "turdagi", "turi"},
			"sku":            {"sku", "article", "artikul", "code", "kod", "product_code"},
			"sell_price":     {"sell_price", "price", "narx", "narxi", "sotish narxi", "selling_price"},
			"cost_price_som": {"cost_price", "cost", "tan narx", "tan_narx", "cost_som", "tan_narxi"},
			"description":    {"description", "tavsif", "notes", "izoh", "desc"},
			"has_sizes":      {"has_sizes", "razmerlar bormi", "sizes", "razmerlar", "sizes_list"},
			"quantity":       {"quantity", "qty", "miqdor", "dona", "stock", "stock_qty", "count"},
			"sizes":          {"sizes_list", "razmer_list", "o'lchamlar", "size_values"},
		},
	}
}

// ParseExcelFile parses a product Excel file and returns parsed products
func (p *ExcelProductParser) ParseExcelFile(filePath string) ([]*ParsedProduct, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get the first sheet name
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	// Get all rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file must have at least a header row and one data row")
	}

	headerRow := rows[0]
	fieldIndices := p.mapHeaderToFields(headerRow)

	var parsedProducts []*ParsedProduct

	// Process each data row
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]

		// Skip empty rows
		isEmptyRow := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				isEmptyRow = false
				break
			}
		}
		if isEmptyRow {
			continue
		}

		parsedProduct := p.parseRow(row, fieldIndices, headerRow)
		if parsedProduct != nil {
			parsedProducts = append(parsedProducts, parsedProduct)
		}
	}

	if len(parsedProducts) == 0 {
		return nil, fmt.Errorf("no valid products found in Excel file")
	}

	return parsedProducts, nil
}

// mapHeaderToFields maps Excel headers to product fields
func (p *ExcelProductParser) mapHeaderToFields(headerRow []string) map[int]string {
	indices := make(map[int]string)

	for colIdx, headerCell := range headerRow {
		headerLower := strings.ToLower(strings.TrimSpace(headerCell))

		// Try to find matching field
		for field, possibleHeaders := range p.fieldMappings {
			for _, possibleHeader := range possibleHeaders {
				if strings.EqualFold(headerLower, possibleHeader) ||
					strings.Contains(headerLower, strings.ToLower(possibleHeader)) {
					indices[colIdx] = field
					goto nextHeader
				}
			}
		}

		// If not matched, keep as extra field with column header as identifier
		indices[colIdx] = "extra:" + headerLower

	nextHeader:
	}

	return indices
}

// parseRow parses a single row and creates a ParsedProduct
func (p *ExcelProductParser) parseRow(row []string, fieldIndices map[int]string, headerRow []string) *ParsedProduct {
	pp := &ParsedProduct{
		Product:    &models.Product{Status: models.ProductStatusActive},
		Errors:     []string{},
		Warnings:   []string{},
		ExtraCells: make(map[string]interface{}),
	}

	fieldData := make(map[string]string)

	// Extract all field values from row
	for colIdx, fieldName := range fieldIndices {
		var cellValue string
		if colIdx < len(row) {
			cellValue = strings.TrimSpace(row[colIdx])
		}

		if strings.HasPrefix(fieldName, "extra:") {
			headerName := strings.TrimPrefix(fieldName, "extra:")
			if cellValue != "" {
				pp.ExtraCells[headerName] = cellValue
			}
		} else if cellValue != "" {
			fieldData[fieldName] = cellValue
		}
	}

	// Validate required fields
	for _, required := range p.requiredFields {
		if _, exists := fieldData[required]; !exists || fieldData[required] == "" {
			pp.Errors = append(pp.Errors, fmt.Sprintf("Missing required field: %s", required))
		}
	}

	// If critical errors, return early
	if len(pp.Errors) > 0 {
		return pp
	}

	// Populate product fields
	pp.Product.Name = fieldData["name"]

	if brand, ok := fieldData["brand"]; ok {
		pp.Product.Brand = strings.ToUpper(brand)
	}

	if country, ok := fieldData["country"]; ok {
		pp.Product.Country = country
	}

	if category, ok := fieldData["category"]; ok {
		pp.Product.Category = category
	}

	if sku, ok := fieldData["sku"]; ok {
		pp.Product.SKU = sku
	}

	if description, ok := fieldData["description"]; ok {
		pp.Product.Description = description
	}

	// Parse prices
	if sellPriceStr, ok := fieldData["sell_price"]; ok {
		if price, err := parsePrice(sellPriceStr); err == nil {
			pp.Product.SellPrice = price
		} else {
			pp.Errors = append(pp.Errors, fmt.Sprintf("Invalid sell price: %s", sellPriceStr))
		}
	}

	if costPriceStr, ok := fieldData["cost_price_som"]; ok {
		if price, err := parsePrice(costPriceStr); err == nil {
			pp.Product.CostPriceSom = price
		} else {
			pp.Errors = append(pp.Errors, fmt.Sprintf("Invalid cost price: %s", costPriceStr))
		}
	}

	// Parse sizes
	if hasSpaces, ok := fieldData["has_sizes"]; ok {
		pp.Product.HasSizes = parseBool(hasSpaces)
	}

	// Parse stock quantity
	stockQty := 0
	if qtyStr, ok := fieldData["quantity"]; ok {
		if q, err := strconv.Atoi(strings.TrimSpace(qtyStr)); err == nil && q >= 0 {
			stockQty = q
		}
	}

	// Build stock items from sizes or single quantity
	if sizesStr, ok := fieldData["sizes"]; ok && sizesStr != "" {
		parts := strings.FieldsFunc(sizesStr, func(r rune) bool {
			return r == ',' || r == '/' || r == '|'
		})
		for _, part := range parts {
			s := strings.TrimSpace(strings.ToUpper(part))
			if s != "" {
				pp.Product.Stock = append(pp.Product.Stock, models.StockItem{
					Size:              s,
					QuantityAvailable: stockQty,
				})
			}
		}
		if len(pp.Product.Stock) > 0 {
			pp.Product.HasSizes = true
		}
	} else if stockQty > 0 {
		pp.Product.Stock = append(pp.Product.Stock, models.StockItem{
			QuantityAvailable: stockQty,
		})
	}

	// Store any extra fields not in our struct
	pp.Product.ExtraFields = make(map[string]string)
	for key, val := range pp.ExtraCells {
		pp.Product.ExtraFields[key] = fmt.Sprintf("%v", val)
	}

	return pp
}

// parsePrice extracts numeric price from string with various formats
func parsePrice(priceStr string) (float64, error) {
	// Remove common separators and non-numeric characters except decimal point
	cleaned := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			return r
		}
		return -1
	}, priceStr)

	// Replace comma with dot for decimal separator
	cleaned = strings.ReplaceAll(cleaned, ",", ".")

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("invalid price value")
	}

	return price, nil
}

// parseBool parses various boolean representations
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "ha" || s == "1" || s == "y" ||
		s == " ha," || s == "bor" || s == "✅" || s == "on"
}
