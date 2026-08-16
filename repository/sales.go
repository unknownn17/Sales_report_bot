package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-sales-bot/db"
	"telegram-sales-bot/models"
)

type SaleRepo struct {
	col *mongo.Collection
}

func NewSaleRepo(database *mongo.Database) *SaleRepo {
	return &SaleRepo{
		col: database.Collection(db.ColSales),
	}
}

func (r *SaleRepo) InsertSale(ctx context.Context, s *models.Sale) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.ID.IsZero() {
		s.ID = primitive.NewObjectID()
	}
	if s.SoldAt.IsZero() {
		s.SoldAt = time.Now()
	}

	res, err := r.col.InsertOne(timeoutCtx, s)
	if err != nil {
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		s.ID = oid
	}

	return nil
}

func (r *SaleRepo) FindSaleByID(ctx context.Context, id primitive.ObjectID) (*models.Sale, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var s models.Sale
	err := r.col.FindOne(timeoutCtx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *SaleRepo) FindDebtors(ctx context.Context) ([]models.Sale, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{
		"payment_status": bson.M{"$ne": models.PaymentStatusPaid},
		"amount_due":     bson.M{"$gt": 0},
	}
	findOpts := options.Find().SetSort(bson.D{
		{Key: "due_date", Value: 1},
		{Key: "amount_due", Value: -1},
	})

	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var sales []models.Sale
	if err := cursor.All(timeoutCtx, &sales); err != nil {
		return nil, err
	}

	return sales, nil
}

func (r *SaleRepo) FindDueSoon(ctx context.Context, withinDays int) ([]models.Sale, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dueDateLimit := time.Now().AddDate(0, 0, withinDays)

	filter := bson.M{
		"payment_status": bson.M{"$ne": models.PaymentStatusPaid},
		"amount_due":     bson.M{"$gt": 0},
		"due_date": bson.M{
			"$ne":  nil,
			"$lte": dueDateLimit,
		},
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}})

	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var sales []models.Sale
	if err := cursor.All(timeoutCtx, &sales); err != nil {
		return nil, err
	}

	return sales, nil
}

func (r *SaleRepo) UpdateSalePayment(ctx context.Context, saleID primitive.ObjectID, amountPaid, amountDue float64, status string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"amount_paid":    amountPaid,
			"amount_due":     amountDue,
			"payment_status": status,
		},
	}

	_, err := r.col.UpdateOne(timeoutCtx, bson.M{"_id": saleID}, update)
	return err
}

type ProfitReportResult struct {
	ReceivedProfit float64
	PendingProfit  float64
	TotalSold      int
	CashCount      int
	CardCount      int
	CreditCount    int
	TopProducts    []TopProduct
}

type TopProduct struct {
	ProductID   primitive.ObjectID
	ProductName string
	TotalQty    int
}

func (r *SaleRepo) ProfitReport(ctx context.Context, from, to time.Time) (*ProfitReportResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	matchStage := bson.D{{Key: "$match", Value: bson.M{
		"sold_at": bson.M{
			"$gte": from,
			"$lt":  to,
		},
	}}}

	groupStage := bson.D{{Key: "$group", Value: bson.M{
		"_id": nil,
		"received_profit": bson.M{"$sum": bson.M{
			"$multiply": bson.A{
				bson.M{"$subtract": bson.A{"$sell_price_at_sale", "$cost_price_at_sale_som"}},
				bson.M{"$cond": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$multiply": bson.A{"$sell_price_at_sale", "$quantity"}}, 0}},
					0,
					bson.M{"$divide": bson.A{"$amount_paid", bson.M{"$multiply": bson.A{"$sell_price_at_sale", "$quantity"}}}},
				}},
				"$quantity",
			},
		}},
		"pending_profit": bson.M{"$sum": bson.M{
			"$multiply": bson.A{
				bson.M{"$subtract": bson.A{"$sell_price_at_sale", "$cost_price_at_sale_som"}},
				bson.M{"$cond": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$multiply": bson.A{"$sell_price_at_sale", "$quantity"}}, 0}},
					0,
					bson.M{"$divide": bson.A{"$amount_due", bson.M{"$multiply": bson.A{"$sell_price_at_sale", "$quantity"}}}},
				}},
				"$quantity",
			},
		}},
		"total_sold":   bson.M{"$sum": "$quantity"},
		"cash_count":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$payment_type", models.PaymentTypeCash}}, 1, 0}}},
		"card_count":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$payment_type", models.PaymentTypeCard}}, 1, 0}}},
		"credit_count": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$payment_type", models.PaymentTypeCredit}}, 1, 0}}},
	}}}

	cursorStats, err := r.col.Aggregate(timeoutCtx, mongo.Pipeline{matchStage, groupStage})
	if err != nil {
		return nil, err
	}
	defer cursorStats.Close(timeoutCtx)

	var statsResult []struct {
		ReceivedProfit float64 `bson:"received_profit"`
		PendingProfit  float64 `bson:"pending_profit"`
		TotalSold      int     `bson:"total_sold"`
		CashCount      int     `bson:"cash_count"`
		CardCount      int     `bson:"card_count"`
		CreditCount    int     `bson:"credit_count"`
	}

	if err := cursorStats.All(timeoutCtx, &statsResult); err != nil {
		return nil, err
	}

	result := &ProfitReportResult{}
	if len(statsResult) > 0 {
		sr := statsResult[0]
		result.ReceivedProfit = sr.ReceivedProfit
		result.PendingProfit = sr.PendingProfit
		result.TotalSold = sr.TotalSold
		result.CashCount = sr.CashCount
		result.CardCount = sr.CardCount
		result.CreditCount = sr.CreditCount
	}

	topGroupStage := bson.D{{Key: "$group", Value: bson.M{
		"_id": bson.M{
			"pid":   "$product_id",
			"pname": "$product_name",
		},
		"total_qty": bson.M{"$sum": "$quantity"},
	}}}

	sortStage := bson.D{{Key: "$sort", Value: bson.M{"total_qty": -1}}}
	limitStage := bson.D{{Key: "$limit", Value: 5}}

	cursorTop, err := r.col.Aggregate(timeoutCtx, mongo.Pipeline{matchStage, topGroupStage, sortStage, limitStage})
	if err != nil {
		return nil, err
	}
	defer cursorTop.Close(timeoutCtx)

	var topResults []struct {
		ID struct {
			PID   primitive.ObjectID `bson:"pid"`
			PName string             `bson:"pname"`
		} `bson:"_id"`
		TotalQty int `bson:"total_qty"`
	}

	if err := cursorTop.All(timeoutCtx, &topResults); err != nil {
		return nil, err
	}

	for _, tr := range topResults {
		result.TopProducts = append(result.TopProducts, TopProduct{
			ProductID:   tr.ID.PID,
			ProductName: tr.ID.PName,
			TotalQty:    tr.TotalQty,
		})
	}

	return result, nil
}

// FindSalesInRange returns all individual sale records within a date range, sorted by date.
func (r *SaleRepo) FindSalesInRange(ctx context.Context, from, to time.Time) ([]models.Sale, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	filter := bson.M{
		"sold_at": bson.M{
			"$gte": from,
			"$lt":  to,
		},
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "sold_at", Value: -1}})
	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var sales []models.Sale
	if err := cursor.All(timeoutCtx, &sales); err != nil {
		return nil, err
	}

	return sales, nil
}
