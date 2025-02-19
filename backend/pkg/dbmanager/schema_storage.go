package dbmanager

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"neobase-ai/pkg/redis"
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
	// Marshal to JSON
	data, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %v", err)
	}

	// Compress
	compressed, err := s.compress(data)
	if err != nil {
		return fmt.Errorf("failed to compress schema: %v", err)
	}

	// Encrypt returns a string
	encryptedStr, err := s.encryption.Encrypt(compressed)
	if err != nil {
		return fmt.Errorf("failed to encrypt schema: %v", err)
	}

	// Store in Redis - convert string to []byte
	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	return s.redisRepo.Set(key, []byte(encryptedStr), schemaTTL, ctx)
}

func (s *SchemaStorageService) Retrieve(ctx context.Context, chatID string) (*SchemaStorage, error) {
	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	encryptedBytes, err := s.redisRepo.Get(key, ctx)
	if err != nil {
		return nil, err
	}

	// Convert []byte to string for decryption
	encryptedStr := string(encryptedBytes)

	// Decrypt takes string, returns []byte
	decrypted, err := s.encryption.Decrypt(encryptedStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt schema: %v", err)
	}

	// Decompress
	decompressed, err := s.decompress(decrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress schema: %v", err)
	}

	// Unmarshal
	var storage SchemaStorage
	if err := json.Unmarshal(decompressed, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	return &storage, nil
}

// Compression helpers
func (s *SchemaStorageService) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress data: %v", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compressor: %v", err)
	}

	return buf.Bytes(), nil
}

func (s *SchemaStorageService) decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %v", err)
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %v", err)
	}

	return decompressed, nil
}
