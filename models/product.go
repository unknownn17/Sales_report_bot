package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	SKU          string             `bson:"sku,omitempty"`
	Name         string             `bson:"name"`
	Brand        string             `bson:"brand,omitempty"`
	Country      string             `bson:"country,omitempty"` // Import country: Turkiya, Xitoy, etc.
	Category     string             `bson:"category,omitempty"`
	HasSizes     bool               `bson:"has_sizes"`
	CostPriceSom float64            `bson:"cost_price_som"` // Cost price in Uzbek so'm (directly entered)
	SellPrice    float64            `bson:"sell_price"`     // Selling price in Uzbek so'm
	Description  string             `bson:"description,omitempty"`
	PhotoFileID  string             `bson:"photo_file_id,omitempty"`
	Status       string             `bson:"status"` // active | out_of_stock | archived
	Stock        []StockItem        `bson:"stock"`  // embedded
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
	AddedBy      int64              `bson:"added_by"`
}

type StockItem struct {
	Size              string `bson:"size,omitempty"` // empty/omitted when HasSizes = false
	QuantityAvailable int    `bson:"quantity_available"`
}

const (
	ProductStatusActive     = "active"
	ProductStatusOutOfStock = "out_of_stock"
	ProductStatusArchived   = "archived"
)

var DefaultCountries = []string{"Turkiya 🇹🇷", "Xitoy 🇨🇳", "BAA (Dubay) 🇦🇪", "O'zbekiston 🇺🇿", "Qirg'iziston 🇰🇬", "Vyetnam 🇻🇳"}
