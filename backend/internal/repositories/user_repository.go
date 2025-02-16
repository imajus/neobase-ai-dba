package repositories

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
	CreateUserSignUpSecret(secret string) (*models.UserSignupSecret, error)
	ValidateUserSignupSecret(secret string) bool
	DeleteUserSignupSecret(secret string) error
}

type userRepository struct {
	userCollection             *mongo.Collection
	userSignupSecretCollection *mongo.Collection
}

func NewUserRepository(mongoClient *mongodb.MongoDBClient) UserRepository {
	return &userRepository{
		userCollection:             mongoClient.GetCollectionByName("users"),
		userSignupSecretCollection: mongoClient.GetCollectionByName("userSignupSecrets"),
	}
}

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.userCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(user *models.User) error {
	if user.ID.IsZero() {
		user.Base = models.NewBase()
	}
	_, err := r.userCollection.InsertOne(context.Background(), user)
	return err
}

func (r *userRepository) CreateUserSignUpSecret(secret string) (*models.UserSignupSecret, error) {
	signupSecret := models.NewUserSignupSecret(secret)
	_, err := r.userSignupSecretCollection.InsertOne(context.Background(), signupSecret)
	if err != nil {
		return nil, err
	}
	return signupSecret, nil
}

func (r *userRepository) ValidateUserSignupSecret(secret string) bool {
	var signupSecret models.UserSignupSecret
	err := r.userSignupSecretCollection.FindOne(context.Background(), bson.M{"secret": secret}).Decode(&signupSecret)
	return err == nil
}

func (r *userRepository) DeleteUserSignupSecret(secret string) error {
	_, err := r.userSignupSecretCollection.DeleteOne(context.Background(), bson.M{"secret": secret})
	return err
}
