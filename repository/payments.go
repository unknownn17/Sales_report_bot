package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-sales-bot/db"
	"telegram-sales-bot/models"
)

type PaymentRepo struct {
	col *mongo.Collection
}

func NewPaymentRepo(database *mongo.Database) *PaymentRepo {
	return &PaymentRepo{
		col: database.Collection(db.ColPayments),
	}
}

func (r *PaymentRepo) InsertPayment(ctx context.Context, p *models.Payment) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.ID.IsZero() {
		p.ID = primitive.NewObjectID()
	}
	if p.PaidAt.IsZero() {
		p.PaidAt = time.Now()
	}

	res, err := r.col.InsertOne(timeoutCtx, p)
	if err != nil {
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		p.ID = oid
	}

	return nil
}

func (r *PaymentRepo) FindPaymentsBySaleID(ctx context.Context, saleID primitive.ObjectID) ([]models.Payment, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	findOpts := options.Find().SetSort(bson.D{{Key: "paid_at", Value: -1}})
	cursor, err := r.col.Find(timeoutCtx, bson.M{"sale_id": saleID}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var payments []models.Payment
	if err := cursor.All(timeoutCtx, &payments); err != nil {
		return nil, err
	}

	return payments, nil
}
