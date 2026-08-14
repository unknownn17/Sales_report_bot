package repository

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-sales-bot/db"
	"telegram-sales-bot/models"
)

type CategoryRepo struct {
	col *mongo.Collection
}

func NewCategoryRepo(database *mongo.Database) *CategoryRepo {
	return &CategoryRepo{
		col: database.Collection(db.ColCategories),
	}
}

func (r *CategoryRepo) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	findOpts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := r.col.Find(timeoutCtx, bson.M{}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var cats []models.Category
	if err := cursor.All(timeoutCtx, &cats); err != nil {
		return nil, err
	}

	return cats, nil
}

func (r *CategoryRepo) CreateCategory(ctx context.Context, name string, adminID int64) (*models.Category, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	name = strings.TrimSpace(name)
	cat := &models.Category{
		ID:        primitive.NewObjectID(),
		Name:      name,
		CreatedAt: time.Now(),
		CreatedBy: adminID,
	}

	_, err := r.col.InsertOne(timeoutCtx, cat)
	if err != nil {
		return nil, err
	}

	return cat, nil
}

func (r *CategoryRepo) DeleteCategory(ctx context.Context, id primitive.ObjectID) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := r.col.DeleteOne(timeoutCtx, bson.M{"_id": id})
	return err
}

func (r *CategoryRepo) SeedDefaultCategories(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	count, err := r.col.CountDocuments(timeoutCtx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []string{"Pijama", "Futbolka", "Shim", "Ko'ylak", "Krossovka", "Boshqa"}
	for _, name := range defaults {
		_, _ = r.col.InsertOne(timeoutCtx, &models.Category{
			ID:        primitive.NewObjectID(),
			Name:      name,
			CreatedAt: time.Now(),
		})
	}
	return nil
}
