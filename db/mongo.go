package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ColProducts   = "products"
	ColSales      = "sales"
	ColPayments   = "payments"
	ColCategories = "categories"
)

// Connect creates a MongoDB client, connects to the server and pings it.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mongodb client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return client, nil
}

// GetDatabase returns the handle to the database.
func GetDatabase(client *mongo.Client, dbName string) *mongo.Database {
	return client.Database(dbName)
}

// EnsureIndexes creates required indexes for all collections.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 1. Products indexes: status, country, category, name + brand
	productsCol := db.Collection(ColProducts)
	_, err := productsCol.Indexes().CreateMany(timeoutCtx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "country", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "category", Value: 1}},
		},
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "brand", Value: "text"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create products indexes: %w", err)
	}

	// 2. Sales indexes: sold_at, payment_status, product_id
	salesCol := db.Collection(ColSales)
	_, err = salesCol.Indexes().CreateMany(timeoutCtx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "sold_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "payment_status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "product_id", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create sales indexes: %w", err)
	}

	// 3. Payments indexes: sale_id
	paymentsCol := db.Collection(ColPayments)
	_, err = paymentsCol.Indexes().CreateMany(timeoutCtx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "sale_id", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create payments indexes: %w", err)
	}

	// 4. Categories index: name (unique)
	categoriesCol := db.Collection(ColCategories)
	uniqueOpt := true
	_, err = categoriesCol.Indexes().CreateOne(timeoutCtx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: &options.IndexOptions{Unique: &uniqueOpt},
	})
	if err != nil {
		return fmt.Errorf("failed to create categories index: %w", err)
	}

	return nil
}
