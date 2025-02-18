package dbmanager

import (
	"fmt"
	"neobase-ai/internal/utils"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresDriver struct{}

func NewPostgresDriver() DatabaseDriver {
	return &PostgresDriver{}
}

func (d *PostgresDriver) Connect(config ConnectionConfig) (*Connection, error) {
	// If username or password is nil, set it to empty string
	if config.Username == nil {
		config.Username = utils.ToStringPtr("")
	}
	if config.Password == nil {
		config.Password = utils.ToStringPtr("")
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, *config.Username, *config.Password, config.Database)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &Connection{
		DB:       db,
		LastUsed: time.Now(),
		Status:   StatusConnected,
		Config:   config,
	}, nil
}

func (d *PostgresDriver) Disconnect(conn *Connection) error {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *PostgresDriver) Ping(conn *Connection) error {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (d *PostgresDriver) IsAlive(conn *Connection) bool {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return false
	}
	return sqlDB.Ping() == nil
}
