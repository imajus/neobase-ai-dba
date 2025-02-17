package dbmanager

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// SchemaEncryption handles encryption/decryption of schema data
type SchemaEncryption struct {
	key []byte // 32 bytes for AES-256
}

func NewSchemaEncryption(encryptionKey string) (*SchemaEncryption, error) {
	// Key must be 32 bytes for AES-256
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes")
	}

	return &SchemaEncryption{
		key: []byte(encryptionKey),
	}, nil
}

// Encrypt takes a byte slice and returns an encrypted, base64-encoded string
func (se *SchemaEncryption) Encrypt(data []byte) (string, error) {
	block, err := aes.NewCipher(se.key)
	if err != nil {
		return "", err
	}

	// Generate a random nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Create GCM cipher
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Encrypt and append nonce
	ciphertext := aesgcm.Seal(nonce, nonce, data, nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt takes a base64-encoded string and returns the decrypted data
func (se *SchemaEncryption) Decrypt(encodedData string) ([]byte, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(se.key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < 12 {
		return nil, fmt.Errorf("data too short")
	}

	nonce := data[:12]
	ciphertext := data[12:]

	// Decrypt
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
