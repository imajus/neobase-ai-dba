package models

type UserSignupSecret struct {
	ID     string `bson:"_id,omitempty"`
	Secret string `bson:"secret"`
	Base
}
