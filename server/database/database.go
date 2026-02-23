package database

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"piliminusb/config"
)

var DB *gorm.DB

func Init() {
	cfg := config.Get()
	var err error
	DB, err = gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
}
