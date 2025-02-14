package mongodb

type MongoDbConfigModel struct {
	ConnectionUrl string `json:"connection_url"`
	DatabaseName  string `json:"database_name"`
}
