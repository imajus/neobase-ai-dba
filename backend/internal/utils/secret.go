package utils

import "github.com/google/uuid"

func GenerateSecret() string {
	return uuid.New().String()
}
