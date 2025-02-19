package repositories

import (
	"context"
	"log"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatRepository interface {
	Create(chat *models.Chat) error
	Update(id primitive.ObjectID, chat *models.Chat) error
	Delete(id primitive.ObjectID) error
	FindByID(id primitive.ObjectID) (*models.Chat, error)
	FindByUserID(userID primitive.ObjectID, page, pageSize int) ([]*models.Chat, int64, error)
	CreateMessage(message *models.Message) error
	UpdateMessage(id primitive.ObjectID, message *models.Message) error
	DeleteMessages(chatID primitive.ObjectID) error
	FindMessagesByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error)
	FindMessageByID(id primitive.ObjectID) (*models.Message, error)
}

type chatRepository struct {
	chatCollection    *mongo.Collection
	messageCollection *mongo.Collection
}

func NewChatRepository(mongoClient *mongodb.MongoDBClient) ChatRepository {
	return &chatRepository{
		chatCollection:    mongoClient.GetCollectionByName("chats"),
		messageCollection: mongoClient.GetCollectionByName("messages"),
	}
}

func (r *chatRepository) Create(chat *models.Chat) error {
	_, err := r.chatCollection.InsertOne(context.Background(), chat)
	return err
}

func (r *chatRepository) Update(id primitive.ObjectID, chat *models.Chat) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": chat}
	_, err := r.chatCollection.UpdateOne(context.Background(), filter, update)
	return err
}

func (r *chatRepository) Delete(id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	_, err := r.chatCollection.DeleteOne(context.Background(), filter)
	return err
}

func (r *chatRepository) FindByID(id primitive.ObjectID) (*models.Chat, error) {
	var chat models.Chat
	err := r.chatCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&chat)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &chat, err
}

func (r *chatRepository) FindByUserID(userID primitive.ObjectID, page, pageSize int) ([]*models.Chat, int64, error) {
	var chats []*models.Chat
	filter := bson.M{"user_id": userID}

	// Get total count
	total, err := r.chatCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.chatCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &chats)
	return chats, total, err
}

func (r *chatRepository) CreateMessage(message *models.Message) error {
	log.Printf("CreateMessage -> message: %v", message)
	_, err := r.messageCollection.InsertOne(context.Background(), message)
	return err
}

func (r *chatRepository) UpdateMessage(id primitive.ObjectID, message *models.Message) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": message}
	_, err := r.messageCollection.UpdateOne(context.Background(), filter, update)
	return err
}

func (r *chatRepository) DeleteMessages(chatID primitive.ObjectID) error {
	filter := bson.M{"chat_id": chatID}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)
	return err
}

func (r *chatRepository) FindMessagesByChat(chatID primitive.ObjectID, page, pageSize int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	filter := bson.M{"chat_id": chatID}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	// Setup pagination
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "created_at", Value: 1}}) // Ascending order for messages

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *chatRepository) FindMessageByID(id primitive.ObjectID) (*models.Message, error) {
	var message models.Message
	err := r.messageCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&message)
	return &message, err
}
