package repositories

import (
	"context"
	"neobase-ai/internal/models"
	"neobase-ai/pkg/mongodb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LLMMessageRepository interface {
	// Message operations
	CreateMessage(msg *models.LLMMessage) error
	UpdateMessage(id primitive.ObjectID, message *models.LLMMessage) error
	FindMessageByID(id primitive.ObjectID) (*models.LLMMessage, error)
	FindMessageByChatMessageID(messageID primitive.ObjectID) (*models.LLMMessage, error)
	FindMessagesByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, int64, error)
	DeleteMessagesByChatID(chatID primitive.ObjectID, dontDeleteSystemMessages bool) error
	DeleteMessagesByRole(userID primitive.ObjectID, chatID primitive.ObjectID, role string) error
	GetByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, error)
}

type llmMessageRepository struct {
	messageCollection *mongo.Collection
	streamCollection  *mongo.Collection
}

func NewLLMMessageRepository(mongoClient *mongodb.MongoDBClient) LLMMessageRepository {
	return &llmMessageRepository{
		messageCollection: mongoClient.GetCollectionByName("llm_messages"),
		streamCollection:  mongoClient.GetCollectionByName("llm_message_streams"),
	}
}

// Message operations
func (r *llmMessageRepository) CreateMessage(msg *models.LLMMessage) error {
	_, err := r.messageCollection.InsertOne(context.Background(), msg)
	return err
}

func (r *llmMessageRepository) UpdateMessage(id primitive.ObjectID, message *models.LLMMessage) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": message}
	_, err := r.messageCollection.UpdateOne(context.Background(), filter, update)
	return err
}

func (r *llmMessageRepository) FindMessageByID(id primitive.ObjectID) (*models.LLMMessage, error) {
	var message models.LLMMessage
	err := r.messageCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&message)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &message, err
}

func (r *llmMessageRepository) FindMessagesByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, int64, error) {
	var messages []*models.LLMMessage
	filter := bson.M{"chat_id": chatID}

	// Get total count
	total, err := r.messageCollection.CountDocuments(context.Background(), filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.messageCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, total, err
}

func (r *llmMessageRepository) DeleteMessagesByChatID(chatID primitive.ObjectID, dontDeleteSystemMessages bool) error {
	filter := bson.M{"chat_id": chatID}
	if dontDeleteSystemMessages {
		filter["role"] = bson.M{"$ne": "system"}
	}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)
	return err
}

func (r *llmMessageRepository) GetByChatID(chatID primitive.ObjectID) ([]*models.LLMMessage, error) {
	var messages []*models.LLMMessage
	filter := bson.M{"chat_id": chatID}

	cursor, err := r.messageCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	err = cursor.All(context.Background(), &messages)
	return messages, err
}

// FindMessageByChatMessageID finds a message by the chat message ID(original message id)
func (r *llmMessageRepository) FindMessageByChatMessageID(messageID primitive.ObjectID) (*models.LLMMessage, error) {
	var message models.LLMMessage
	err := r.messageCollection.FindOne(context.Background(), bson.M{"message_id": messageID}).Decode(&message)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &message, err
}

func (r *llmMessageRepository) DeleteMessagesByRole(userID primitive.ObjectID, chatID primitive.ObjectID, role string) error {
	filter := bson.M{"user_id": userID, "chat_id": chatID, "role": role}
	_, err := r.messageCollection.DeleteMany(context.Background(), filter)
	return err
}
