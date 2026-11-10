package user

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

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
	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permission{}, &models.Session{}); err != nil {
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

	users, err := svc.List(UserListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserService_List_FilterByKeywordAndRole(t *testing.T) {
	db := newUserServiceTestDB(t)
	adminRole := seedRoleForService(t, db, "admin")
	userRole := seedRoleForService(t, db, "user")
	alice := seedUserForService(t, db, "alice", "uuid-alice")
	bob := seedUserForService(t, db, "bob", "uuid-bob")
	if err := db.Model(&alice).Association("Roles").Replace(&[]models.Role{adminRole}); err != nil {
		t.Fatalf("attach alice role failed: %v", err)
	}
	if err := db.Model(&bob).Association("Roles").Replace(&[]models.Role{userRole}); err != nil {
		t.Fatalf("attach bob role failed: %v", err)
	}

	svc := NewUserService(db)
	users, err := svc.List(UserListFilter{
		Keyword:  "ali",
		RoleCode: "admin",
	})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 filtered user, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Fatalf("expected alice, got %q", users[0].Username)
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

	_, err := svc.UpdateByUUID("uuid-1", UserUpdateInput{RoleCodes: &[]string{"admin"}})
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
		RoleCodes:   &[]string{"admin"},
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

func TestUserService_UpdateByUUID_ClearRoles(t *testing.T) {
	db := newUserServiceTestDB(t)
	user := seedUserForService(t, db, "u1", "uuid-1")
	adminRole := seedRoleForService(t, db, "admin")
	if err := db.Model(&user).Association("Roles").Replace(&[]models.Role{adminRole}); err != nil {
		t.Fatalf("seed roles failed: %v", err)
	}

	svc := NewUserService(db)
	empty := []string{}
	if _, err := svc.UpdateByUUID(user.UUID, UserUpdateInput{RoleCodes: &empty}); err != nil {
		t.Fatalf("UpdateByUUID clear roles failed: %v", err)
	}

	out, err := svc.GetByUUID(user.UUID)
	if err != nil {
		t.Fatalf("GetByUUID failed: %v", err)
	}
	if len(out.Roles) != 0 {
		t.Fatalf("expected no roles, got %+v", out.Roles)
	}
}

func TestUserService_ProfileNotFound(t *testing.T) {
	svc := NewUserService(newUserServiceTestDB(t))
	if _, err := svc.GetProfileById("9999"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("profile: got %v", err)
	}
	if _, err := svc.GetProfilePublic("9999"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("public profile: got %v", err)
	}
}

func TestUserService_InvalidRoleRollsBackAllFields(t *testing.T) {
	for _, byID := range []bool{false, true} {
		t.Run(strconv.FormatBool(byID), func(t *testing.T) {
			db := newUserServiceTestDB(t)
			user := seedUserForService(t, db, "unchanged", "unchanged-uuid")
			nickname := "must-not-save"
			permissions := "admin"
			input := UserUpdateInput{Nickname: &nickname, Permissions: &permissions, RoleCodes: &[]string{"missing-role"}}
			var err error
			if byID {
				_, err = NewUserService(db).UpdateByID(strconv.Itoa(user.Id), input)
			} else {
				_, err = NewUserService(db).UpdateByUUID(user.UUID, input)
			}
			if !errors.Is(err, ErrUserRoleNotFound) {
				t.Fatalf("expected invalid role: %v", err)
			}
			var saved models.User
			db.First(&saved, user.Id)
			if saved.Nickname != "" || saved.Permissions != "" {
				t.Fatal("invalid role caused a partial update")
			}
		})
	}
}

func TestUserService_DisablingAccountRevokesSessions(t *testing.T) {
	db := newUserServiceTestDB(t)
	user := seedUserForService(t, db, "disabled-user", "disabled-uuid")
	expiry := time.Now().Add(time.Hour)
	if err := db.Create(&models.Session{UserId: user.Id, SessionId: "active-session", ExpiresAt: &expiry}).Error; err != nil {
		t.Fatal(err)
	}
	status := "disabled"
	if _, err := NewUserService(db).UpdateByID(strconv.Itoa(user.Id), UserUpdateInput{Status: &status}); err != nil {
		t.Fatal(err)
	}
	var session models.Session
	if err := db.First(&session).Error; err != nil {
		t.Fatal(err)
	}
	if session.RevokedAt == nil {
		t.Fatal("disabled user's session remains valid")
	}
}
