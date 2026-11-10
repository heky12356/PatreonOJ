// admin grants administrator permission to an existing, registered account.
package main

import (
	"flag"
	"log"
	"strings"

	"patreonoj/internal/config"
	"patreonoj/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	username := flag.String("username", "", "existing account username")
	configPath := flag.String("config", "config.yaml", "configuration file")
	flag.Parse()
	if strings.TrimSpace(*username) == "" {
		log.Fatal("usage: admin -username <registered username> [-config config.yaml]")
	}
	if err := config.InitConfig(*configPath); err != nil {
		log.Fatal(err)
	}
	if err := models.InitDB(); err != nil {
		log.Fatal(err)
	}
	sqlDB, err := models.DB.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if err := grantAdmin(models.DB, *username); err != nil {
		log.Fatalf("grant administrator permission failed: %v", err)
	}
	log.Printf("administrator permission granted to %s; sign in again to refresh the UI", *username)
}

func grantAdmin(db *gorm.DB, username string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
			return err
		}
		for _, permission := range strings.Split(user.Permissions, ",") {
			if strings.TrimSpace(permission) == "admin" {
				return nil
			}
		}
		permissions := strings.Trim(user.Permissions, " ,")
		if permissions != "" {
			permissions += ","
		}
		return tx.Model(&user).Update("permissions", permissions+"admin").Error
	})
}
