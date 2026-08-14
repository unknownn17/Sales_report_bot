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

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductNotFound   = errors.New("product not found")
)

type ProductRepo struct {
	col *mongo.Collection
}

func NewProductRepo(database *mongo.Database) *ProductRepo {
	return &ProductRepo{
		col: database.Collection(db.ColProducts),
	}
}

func (r *ProductRepo) InsertProduct(ctx context.Context, p *models.Product) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.ID.IsZero() {
		p.ID = primitive.NewObjectID()
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	res, err := r.col.InsertOne(timeoutCtx, p)
	if err != nil {
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		p.ID = oid
	}

	return nil
}

func (r *ProductRepo) FindProductByID(ctx context.Context, id primitive.ObjectID) (*models.Product, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var p models.Product
	err := r.col.FindOne(timeoutCtx, bson.M{"_id": id}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepo) FindActiveProducts(ctx context.Context, search string, page, pageSize int) ([]models.Product, int64, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"status": models.ProductStatusActive}
	if search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"brand": bson.M{"$regex": search, "$options": "i"}},
			{"category": bson.M{"$regex": search, "$options": "i"}},
			{"country": bson.M{"$regex": search, "$options": "i"}},
			{"$text": bson.M{"$search": search}},
		}
	}

	total, err := r.col.CountDocuments(timeoutCtx, filter)
	if err != nil {
		return nil, 0, err
	}

	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(timeoutCtx)

	var products []models.Product
	if err := cursor.All(timeoutCtx, &products); err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *ProductRepo) FindAllActiveWithStock(ctx context.Context) ([]models.Product, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"status": models.ProductStatusActive}
	findOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var products []models.Product
	if err := cursor.All(timeoutCtx, &products); err != nil {
		return nil, err
	}

	return products, nil
}

// FindDistinctCountries returns all distinct country values for active products
func (r *ProductRepo) FindDistinctCountries(ctx context.Context) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"status": models.ProductStatusActive}
	values, err := r.col.Distinct(timeoutCtx, "country", filter)
	if err != nil {
		return nil, err
	}

	var countries []string
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			countries = append(countries, s)
		}
	}
	return countries, nil
}

// FindDistinctCategoriesByCountry returns distinct categories for a given country (or all if country is empty)
func (r *ProductRepo) FindDistinctCategoriesByCountry(ctx context.Context, country string) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"status": models.ProductStatusActive}
	if country != "" {
		filter["country"] = country
	}

	values, err := r.col.Distinct(timeoutCtx, "category", filter)
	if err != nil {
		return nil, err
	}

	var categories []string
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			categories = append(categories, s)
		}
	}
	return categories, nil
}

// FindProductsByCountryAndCategory returns active products filtered by country and category
func (r *ProductRepo) FindProductsByCountryAndCategory(ctx context.Context, country, category string) ([]models.Product, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"status": models.ProductStatusActive}
	if country != "" {
		filter["country"] = country
	}
	if category != "" {
		filter["category"] = category
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(timeoutCtx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var products []models.Product
	if err := cursor.All(timeoutCtx, &products); err != nil {
		return nil, err
	}

	return products, nil
}

// DecrementStock atomically decrements quantity available for the specified size if stock is sufficient.
func (r *ProductRepo) DecrementStock(ctx context.Context, productID primitive.ObjectID, size string, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var filter bson.M
	if size != "" {
		filter = bson.M{
			"_id": productID,
			"stock": bson.M{
				"$elemMatch": bson.M{
					"size":               size,
					"quantity_available": bson.M{"$gte": qty},
				},
			},
		}
	} else {
		filter = bson.M{
			"_id": productID,
			"stock": bson.M{
				"$elemMatch": bson.M{
					"quantity_available": bson.M{"$gte": qty},
				},
			},
		}
	}

	update := bson.M{
		"$inc": bson.M{"stock.$.quantity_available": -qty},
		"$set": bson.M{"updated_at": time.Now()},
	}

	res, err := r.col.UpdateOne(timeoutCtx, filter, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return ErrInsufficientStock
	}

	return nil
}

// IncrementStock atomically rolls back or adds stock back to the specified product.
func (r *ProductRepo) IncrementStock(ctx context.Context, productID primitive.ObjectID, size string, qty int) error {
	if qty <= 0 {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var filter bson.M
	if size != "" {
		filter = bson.M{
			"_id":        productID,
			"stock.size": size,
		}
	} else {
		filter = bson.M{
			"_id": productID,
		}
	}

	var update bson.M
	if size != "" {
		update = bson.M{
			"$inc": bson.M{"stock.$.quantity_available": qty},
			"$set": bson.M{"updated_at": time.Now()},
		}
	} else {
		update = bson.M{
			"$inc": bson.M{"stock.0.quantity_available": qty},
			"$set": bson.M{"updated_at": time.Now()},
		}
	}

	_, err := r.col.UpdateOne(timeoutCtx, filter, update)
	return err
}

func (r *ProductRepo) UpdateProductStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}
	_, err := r.col.UpdateOne(timeoutCtx, bson.M{"_id": id}, update)
	return err
}

// CheckAndUpdateStockStatus checks if all items in stock are 0 and adjusts status.
func (r *ProductRepo) CheckAndUpdateStockStatus(ctx context.Context, productID primitive.ObjectID) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	p, err := r.FindProductByID(timeoutCtx, productID)
	if err != nil || p == nil {
		return err
	}

	totalStock := 0
	for _, it := range p.Stock {
		if it.QuantityAvailable > 0 {
			totalStock += it.QuantityAvailable
		}
	}

	if totalStock == 0 && p.Status == models.ProductStatusActive {
		return r.UpdateProductStatus(timeoutCtx, productID, models.ProductStatusOutOfStock)
	} else if totalStock > 0 && p.Status == models.ProductStatusOutOfStock {
		return r.UpdateProductStatus(timeoutCtx, productID, models.ProductStatusActive)
	}

	return nil
}
