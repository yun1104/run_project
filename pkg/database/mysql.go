package database

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Charset  string
}

var dbInstances []*gorm.DB

func InitMySQL(configs []Config) error {
	for _, cfg := range configs {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.Charset)
		
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			return err
		}
		
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		
		sqlDB.SetMaxIdleConns(20)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
		
		dbInstances = append(dbInstances, db)
	}
	return nil
}

func GetDB(userID int64) *gorm.DB {
	if len(dbInstances) == 0 {
		return nil
	}
	index := userID % int64(len(dbInstances))
	return dbInstances[index]
}

func GetDBByIndex(index int) *gorm.DB {
	if index >= len(dbInstances) {
		return nil
	}
	return dbInstances[index]
}
