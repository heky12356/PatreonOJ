package models

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestBuiltinRolesAreIdempotentAndPreserveExistingPolicies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&Role{}, &Permission{}); err != nil {
		t.Fatal(err)
	}
	custom := Permission{Code: "custom.read"}
	db.Create(&custom)
	existing := Role{Name: "moderator", Permissions: []Permission{custom}}
	db.Create(&existing)
	for i := 0; i < 2; i++ {
		if err := EnsureBuiltinRoles(db); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&Role{}).Count(&count)
	if count != 5 {
		t.Fatalf("expected five default roles, got %d", count)
	}
	var administrator Role
	if err := db.Preload("Permissions").Where("name = ?", "admin").First(&administrator).Error; err != nil {
		t.Fatal(err)
	}
	if len(administrator.Permissions) != 1 || administrator.Permissions[0].Code != "admin" {
		t.Fatalf("admin role has no admin permission: %+v", administrator)
	}
	var preserved Role
	db.Preload("Permissions").First(&preserved, existing.ID)
	if len(preserved.Permissions) != 1 || preserved.Permissions[0].Code != "custom.read" {
		t.Fatal("existing policy was overwritten")
	}
	db.Delete(&administrator)
	if err := EnsureBuiltinRoles(db); err != nil {
		t.Fatal(err)
	}
	db.Model(&Role{}).Where("name = ?", "admin").Count(&count)
	if count != 0 {
		t.Fatal("deleted role was restored without authorization")
	}
}
