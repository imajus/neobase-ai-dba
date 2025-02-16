package models

type User struct {
	Username string `bson:"username" json:"username"`
	Password string `bson:"password" json:"-"`
	Base     `bson:",inline"`
}

func NewUser(username, password string) *User {
	return &User{
		Username: username,
		Password: password,
		Base:     NewBase(),
	}
}
