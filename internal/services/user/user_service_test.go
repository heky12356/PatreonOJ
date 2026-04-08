package user

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newUserServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permission{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedUserForService(t *testing.T, db *gorm.DB, username, uuid string) models.User {
	t.Helper()
	user := models.User{
		Username: username,
		Password: "pwd",
		UUID:     uuid,
		Status:   "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	return user
}

func seedRoleForService(t *testing.T, db *gorm.DB, name string) models.Role {
	t.Helper()
	role := models.Role{Name: name}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed role failed: %v", err)
	}
	return role
}

func TestUserService_List(t *testing.T) {
	db := newUserServiceTestDB(t)
	seedUserForService(t, db, "u1", "uuid-1")
	svc := NewUserService(db)

	users, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserService_GetByUUID_NotFound(t *testing.T) {
	db := newUserServiceTestDB(t)
	svc := NewUserService(db)

	_, err := svc.GetByUUID("missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_UpdateByUUID_StatusInvalid(t *testing.T) {
	db := newUserServiceTestDB(t)
	seedUserForService(t, db, "u1", "uuid-1")
	svc := NewUserService(db)

	bad := "paused"
	_, err := svc.UpdateByUUID("uuid-1", UserUpdateInput{Status: &bad})
	if !errors.Is(err, ErrUserStatusInvalid) {
		t.Fatalf("expected ErrUserStatusInvalid, got %v", err)
	}
}

func TestUserService_UpdateByUUID_RoleNotFound(t *testing.T) {
	db := newUserServiceTestDB(t)
	seedUserForService(t, db, "u1", "uuid-1")
	svc := NewUserService(db)

	_, err := svc.UpdateByUUID("uuid-1", UserUpdateInput{RoleCodes: []string{"admin"}})
	if !errors.Is(err, ErrUserRoleNotFound) {
		t.Fatalf("expected ErrUserRoleNotFound, got %v", err)
	}
}

func TestUserService_UpdateByUUID_Success(t *testing.T) {
	db := newUserServiceTestDB(t)
	user := seedUserForService(t, db, "u1", "uuid-1")
	adminRole := seedRoleForService(t, db, "admin")
	svc := NewUserService(db)

	nickname := "  Alice  "
	status := "active"
	email := "  a@example.com  "
	permissions := " admin,read "
	_, err := svc.UpdateByUUID(user.UUID, UserUpdateInput{
		Nickname:    &nickname,
		Email:       &email,
		Status:      &status,
		Permissions: &permissions,
		RoleCodes:   []string{"admin"},
	})
	if err != nil {
		t.Fatalf("UpdateByUUID failed: %v", err)
	}

	out, err := svc.GetByUUID(user.UUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if out.Nickname != "Alice" || out.Email != "a@example.com" {
		t.Fatalf("unexpected trimmed fields: nickname=%q email=%q", out.Nickname, out.Email)
	}
	if len(out.Roles) != 1 || out.Roles[0].ID != adminRole.ID {
		t.Fatalf("unexpected roles: %+v", out.Roles)
	}
}
