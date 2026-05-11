package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var ErrNilDB = errors.New("database handle is nil")

func Open(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	db, err := gorm.Open(gormmysql.New(gormmysql.Config{
		DSN:               mysqlDSN(cfg),
		DefaultStringSize: 191,
	}), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access mysql database handle: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql database: %w", err)
	}

	return db, nil
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("access mysql database handle: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close mysql database: %w", err)
	}

	return nil
}

type HealthChecker struct {
	db *gorm.DB
}

func NewHealthChecker(db *gorm.DB) HealthChecker {
	return HealthChecker{db: db}
}

func (c HealthChecker) Name() string {
	return "database"
}

func (c HealthChecker) Check(ctx context.Context) error {
	if c.db == nil {
		return ErrNilDB
	}

	sqlDB, err := c.db.DB()
	if err != nil {
		return fmt.Errorf("access mysql database handle: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql database: %w", err)
	}

	return nil
}

func mysqlDSN(cfg config.DatabaseConfig) string {
	location := time.UTC
	mysqlConfig := driver.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		DBName:               cfg.Name,
		ParseTime:            true,
		Loc:                  location,
		Timeout:              cfg.ConnectTimeout,
		ReadTimeout:          15 * time.Second,
		WriteTimeout:         15 * time.Second,
		AllowNativePasswords: true,
		Collation:            "utf8mb4_unicode_ci",
		Params: map[string]string{
			"charset":         "utf8mb4",
			"multiStatements": "false",
		},
	}

	return mysqlConfig.FormatDSN()
}
