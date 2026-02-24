package ioc

import (
	"time"

	"cgoforum/config"
	"cgoforum/internal/repository/dao"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func InitDB(cfg *config.DatabaseConfig, logger *zap.Logger) *gorm.DB {
	logLevel := gormlogger.Warn

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("failed to get sql.DB", zap.Error(err))
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Run migrations
	if err := dao.InitTables(db); err != nil {
		logger.Fatal("failed to init tables", zap.Error(err))
	}

	logger.Info("database initialized successfully")
	return db
}
