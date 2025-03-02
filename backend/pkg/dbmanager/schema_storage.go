package dbmanager

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neobase-ai/pkg/redis"
	"strings"
)

type SchemaStorageService struct {
	redisRepo  redis.IRedisRepositories
	encryption *SchemaEncryption
}

func NewSchemaStorageService(redisRepo redis.IRedisRepositories, encryptionKey string) (*SchemaStorageService, error) {
	encryption, err := NewSchemaEncryption(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema encryption: %v", err)
	}

	return &SchemaStorageService{
		redisRepo:  redisRepo,
		encryption: encryption,
	}, nil
}

func (s *SchemaStorageService) Store(ctx context.Context, chatID string, storage *SchemaStorage) error {
	log.Printf("SchemaStorageService -> Store -> Storing schema for chatID: %s", chatID)

	// Marshal the storage object to JSON
	data, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %v", err)
	}

	// Compress the data
	compressed, err := s.compress(data)
	if err != nil {
		return fmt.Errorf("failed to compress schema: %v", err)
	}

	// Encrypt the compressed data
	encrypted, err := s.encryption.Encrypt(compressed)
	if err != nil {
		return fmt.Errorf("failed to encrypt schema: %v", err)
	}

	// Store in Redis with TTL
	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	if err := s.redisRepo.Set(key, []byte(encrypted), schemaTTL, ctx); err != nil {
		return fmt.Errorf("failed to store schema in Redis: %v", err)
	}

	log.Printf("SchemaStorageService -> Store -> Successfully stored schema for chatID: %s", chatID)
	return nil
}

func (s *SchemaStorageService) Retrieve(ctx context.Context, chatID string) (*SchemaStorage, error) {
	log.Printf("SchemaStorageService -> Retrieve -> Retrieving schema for chatID: %s", chatID)

	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	log.Printf("Getting Redis key: %s", key)
	encryptedData, err := s.redisRepo.Get(key, ctx)
	if err != nil {
		if strings.Contains(err.Error(), "key does not exist") || strings.Contains(err.Error(), "redis: nil") {
			log.Printf("Redis key not found: %s (this is normal for first-time access)", key)
			log.Printf("SchemaStorageService -> Retrieve -> No schema found for chatID %s (expected for first-time schema storage)", chatID)
			return nil, fmt.Errorf("key does not exist: %s", key)
		} else {
			log.Printf("SchemaStorageService -> Retrieve -> Error retrieving schema from Redis: %v", err)
		}
		return nil, fmt.Errorf("failed to get schema from Redis: %v", err)
	}

	// Decrypt the data
	decrypted, err := s.encryption.Decrypt(string(encryptedData))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt schema: %v", err)
	}

	// Decompress the data
	decompressed, err := s.decompress([]byte(decrypted))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress schema: %v", err)
	}

	// Unmarshal into storage object
	var storage SchemaStorage
	if err := json.Unmarshal(decompressed, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	log.Printf("SchemaStorageService -> Retrieve -> Successfully retrieved schema for chatID: %s", chatID)
	return &storage, nil
}

// Compression helpers
func (s *SchemaStorageService) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write compressed data: %v", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compressor: %v", err)
	}

	return buf.Bytes(), nil
}

func (s *SchemaStorageService) decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %v", err)
	}
	defer r.Close()

	return io.ReadAll(r)
}
