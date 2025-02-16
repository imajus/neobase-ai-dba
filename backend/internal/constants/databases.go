package constants

import "time"

const (
	DatabaseTypePostgreSQL = "postgresql"
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeMongoDB    = "mongodb"
	DatabaseTypeRedis      = "redis"
	DatabaseTypeNeo4j      = "neo4j"
	DatabaseTypeClickhouse = "clickhouse"
)

const DatabaseConnctionTTL = 10 * time.Minute // 10 minutes for database connection & schema
