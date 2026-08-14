package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Payment struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	SaleID primitive.ObjectID `bson:"sale_id"`
	Amount float64            `bson:"amount"`
	PaidAt time.Time          `bson:"paid_at"`
	Note   string             `bson:"note,omitempty"`
}
