package models

import "time"

type User struct {
	ID        string    `bson:"_id,omitempty"`
	Username  string    `bson:"username"`
	Password  string    `bson:"password"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
