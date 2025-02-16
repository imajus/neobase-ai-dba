package models

type UserSignupSecret struct {
	Secret string `bson:"secret" json:"secret"`
	Base   `bson:",inline"`
}

func NewUserSignupSecret(secret string) *UserSignupSecret {
	return &UserSignupSecret{
		Secret: secret,
		Base:   NewBase(),
	}
}
