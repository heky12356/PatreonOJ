package main

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"patreonoj/internal/models"
)

func TestGrantAdminPreservesAccountAndIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "existing", UUID: "existing-uuid", Password: "unchanged-hash", Permissions: "read", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := grantAdmin(db, user.Username); err != nil {
			t.Fatal(err)
		}
	}
	var saved models.User
	if err := db.First(&saved, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Permissions != "read,admin" || saved.Password != user.Password || saved.UUID != user.UUID || saved.Status != user.Status {
		t.Fatalf("unexpected account change: %+v", saved)
	}
	if err := grantAdmin(db, "missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing account: %v", err)
	}
}
