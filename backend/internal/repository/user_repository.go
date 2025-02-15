package repository

import (
	"context"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository interface {
	FindByUsername(username string) (*models.User, error)
	Create(user *models.User) error
}

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(mongoClient *mongodb.MongoDBClient) UserRepository {
	return &userRepository{
		collection: mongoClient.GetCollectionByName("users"),
	}
}

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(user *models.User) error {
	_, err := r.collection.InsertOne(context.Background(), user)
	return err
}
