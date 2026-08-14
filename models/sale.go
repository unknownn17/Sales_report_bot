package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Sale struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"`
	ProductID          primitive.ObjectID `bson:"product_id"`
	ProductName        string             `bson:"product_name"` // denormalized snapshot for fast report display
	Size               string             `bson:"size,omitempty"`
	Quantity           int                `bson:"quantity"`
	SellPriceAtSale    float64            `bson:"sell_price_at_sale"`     // per unit, frozen
	CostPriceAtSaleSom float64            `bson:"cost_price_at_sale_som"` // per unit, frozen
	PaymentType        string             `bson:"payment_type"`           // cash | card | credit
	PaymentStatus      string             `bson:"payment_status"`         // paid | partially_paid | unpaid
	AmountPaid         float64            `bson:"amount_paid"`
	AmountDue          float64            `bson:"amount_due"`
	DueDate            *time.Time         `bson:"due_date,omitempty"`
	BuyerName          string             `bson:"buyer_name,omitempty"`
	BuyerContact       string             `bson:"buyer_contact,omitempty"`
	SoldAt             time.Time          `bson:"sold_at"`
	SoldBy             int64              `bson:"sold_by"`
}

const (
	PaymentTypeCash   = "cash"
	PaymentTypeCard   = "card"
	PaymentTypeCredit = "credit"

	PaymentStatusPaid          = "paid"
	PaymentStatusPartiallyPaid = "partially_paid"
	PaymentStatusUnpaid        = "unpaid"
)
