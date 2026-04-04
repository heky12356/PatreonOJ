package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedAuthUser(t *testing.T, db *gorm.DB, username, rawPassword, uuid, status, permissions string) models.User {
	t.Helper()
	user := models.User{
		Username:    username,
		Password:    md5EncodeAuth(rawPassword),
		UUID:        uuid,
		Status:      status,
		Permissions: permissions,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	return user
}

func TestAuthService_LoginSuccess(t *testing.T) {
	db := newAuthTestDB(t)
	seedAuthUser(t, db, "alice", "pass", "uuid-a", "active", "admin,read")
	svc := NewAuthService(db)

	result, err := svc.Login("alice", "pass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if result.Username != "alice" || result.UUID != "uuid-a" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if len(result.Permissions) != 2 {
		t.Fatalf("unexpected permissions length: %d", len(result.Permissions))
	}
}

func TestAuthService_LoginInvalidCredentials(t *testing.T) {
	db := newAuthTestDB(t)
	svc := NewAuthService(db)

	_, err := svc.Login("nobody", "pass")
	if !errors.Is(err, ErrAuthInvalidCredentials) {
		t.Fatalf("expected ErrAuthInvalidCredentials, got %v", err)
	}
}

func TestAuthService_LoginDisabled(t *testing.T) {
	db := newAuthTestDB(t)
	seedAuthUser(t, db, "bob", "pass", "uuid-b", "disabled", "")
	svc := NewAuthService(db)

	_, err := svc.Login("bob", "pass")
	if !errors.Is(err, ErrAuthUserDisabled) {
		t.Fatalf("expected ErrAuthUserDisabled, got %v", err)
	}
}

func TestAuthService_RegisterConflict(t *testing.T) {
	db := newAuthTestDB(t)
	seedAuthUser(t, db, "dup", "pass", "uuid-c", "active", "")
	svc := NewAuthService(db)

	_, err := svc.Register("dup", "pass2")
	if !errors.Is(err, ErrAuthUsernameExists) {
		t.Fatalf("expected ErrAuthUsernameExists, got %v", err)
	}
}

func TestAuthService_RegisterSuccess(t *testing.T) {
	db := newAuthTestDB(t)
	svc := NewAuthService(db)
	svc.uuidGenerator = func() string { return "uuid-new" }

	result, err := svc.Register("new_user", "secret")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if result.UUID != "uuid-new" || result.UserID == 0 {
		t.Fatalf("unexpected register result: %+v", result)
	}
}

func TestAuthService_LogoutNotFound(t *testing.T) {
	db := newAuthTestDB(t)
	svc := NewAuthService(db)

	err := svc.Logout(9999)
	if !errors.Is(err, ErrAuthUserNotFound) {
		t.Fatalf("expected ErrAuthUserNotFound, got %v", err)
	}
}

func TestAuthService_LogoutSuccess(t *testing.T) {
	db := newAuthTestDB(t)
	created := seedAuthUser(t, db, "logout_user", "pass", "uuid-logout", "active", "")
	svc := NewAuthService(db)

	err := svc.Logout(created.Id)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	var user models.User
	if err := db.First(&user, created.Id).Error; err != nil {
		t.Fatalf("query user failed: %v", err)
	}
	if user.UUID != "" {
		t.Fatalf("expected UUID cleared, got %s", user.UUID)
	}
}
