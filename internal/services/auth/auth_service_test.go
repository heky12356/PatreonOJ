package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/models"

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
	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permission{}, &models.Session{}); err != nil {
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
	var session models.Session
	if err := db.Where("session_id = ?", result.SessionID).First(&session).Error; err != nil {
		t.Fatalf("session was not persisted: %v", err)
	}
	if session.UserId == 0 || session.ExpiresAt == nil || !session.ExpiresAt.After(time.Now()) {
		t.Fatal("session has no valid owner or expiry")
	}
	var upgraded models.User
	if err := db.Where("uuid = ?", result.UUID).First(&upgraded).Error; err != nil {
		t.Fatal(err)
	}
	if valid, legacy := verifyPassword(upgraded.Password, "pass"); !valid || legacy {
		t.Fatal("legacy account was not migrated to Argon2id")
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
	svc := NewAuthService(newAuthTestDB(t))
	if err := svc.Logout("missing"); !errors.Is(err, ErrAuthSessionNotFound) {
		t.Fatalf("expected missing-session error, got %v", err)
	}
}

func TestAuthService_LogoutRevokesSessionAndPreservesUser(t *testing.T) {
	db := newAuthTestDB(t)
	user := seedAuthUser(t, db, "logout-user", "pass", "logout-uuid", "active", "")
	svc := NewAuthService(db)
	result, err := svc.Login(user.Username, "pass")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(result.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(result.SessionID); err != nil {
		t.Fatalf("logout must be idempotent: %v", err)
	}
	var session models.Session
	if err := db.Where("session_id = ?", result.SessionID).First(&session).Error; err != nil {
		t.Fatal(err)
	}
	if session.RevokedAt == nil {
		t.Fatal("session was not revoked")
	}
	var saved models.User
	if err := db.First(&saved, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.UUID != user.UUID {
		t.Fatal("logout modified user identity")
	}
}

func TestAuthService_NewAccountUsesSaltedHash(t *testing.T) {
	db := newAuthTestDB(t)
	svc := NewAuthService(db)
	first, err := svc.Register("first-user", "same-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Register("second-user", "same-password")
	if err != nil {
		t.Fatal(err)
	}
	var a, b models.User
	db.First(&a, first.UserID)
	db.First(&b, second.UserID)
	if a.Password == b.Password {
		t.Fatal("equal passwords received the same stored hash")
	}
	if valid, legacy := verifyPassword(a.Password, "same-password"); !valid || legacy {
		t.Fatal("new account hash is not valid Argon2id")
	}
	if _, err := svc.Login("first-user", "wrong"); !errors.Is(err, ErrAuthInvalidCredentials) {
		t.Fatalf("wrong password accepted: %v", err)
	}
	if _, err := svc.Login("first-user", "same-password"); err != nil {
		t.Fatalf("new account cannot log in: %v", err)
	}
}

func TestAuthService_RejectsInvalidInputs(t *testing.T) {
	svc := NewAuthService(newAuthTestDB(t))
	for _, tc := range []struct{ username, password string }{
		{"   ", "pass"}, {"name", ""}, {strings.Repeat("n", 65), "pass"}, {"name", strings.Repeat("p", 4097)},
	} {
		if _, err := svc.Register(tc.username, tc.password); !errors.Is(err, ErrAuthInvalidInput) {
			t.Errorf("register expected input error, got %v", err)
		}
		if _, err := svc.Login(tc.username, tc.password); !errors.Is(err, ErrAuthInvalidInput) {
			t.Errorf("login expected input error, got %v", err)
		}
	}
}

func TestAuthService_SessionGeneratorFailureDoesNotModifyAccount(t *testing.T) {
	db := newAuthTestDB(t)
	user := seedAuthUser(t, db, "session-failure", "pass", "failure-uuid", "active", "")
	svc := NewAuthService(db)
	want := errors.New("entropy unavailable")
	svc.sessionIDGenerator = func() (string, error) { return "", want }
	if _, err := svc.Login(user.Username, "pass"); !errors.Is(err, want) {
		t.Fatalf("expected entropy error, got %v", err)
	}
	var saved models.User
	db.First(&saved, user.Id)
	if saved.Password != user.Password {
		t.Fatal("failed login changed the password hash")
	}
	var count int64
	db.Model(&models.Session{}).Count(&count)
	if count != 0 {
		t.Fatal("failed login created a session")
	}
}

func TestAuthService_SessionInsertFailureRollsBackLegacyUpgrade(t *testing.T) {
	db := newAuthTestDB(t)
	user := seedAuthUser(t, db, "rollback-user", "pass", "rollback-uuid", "active", "")
	if err := db.Migrator().DropTable(&models.Session{}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAuthService(db).Login(user.Username, "pass"); err == nil {
		t.Fatal("expected missing-session-table error")
	}
	var saved models.User
	if err := db.First(&saved, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Password != user.Password {
		t.Fatal("password upgrade escaped failed login transaction")
	}
}

func TestAuthService_LoginReturnsRolePermissions(t *testing.T) {
	db := newAuthTestDB(t)
	user := seedAuthUser(t, db, "role-user", "pass", "role-uuid", "active", "read")
	role := models.Role{Name: "administrator", Permissions: []models.Permission{{Code: "admin"}, {Code: "read"}}}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user).Association("Roles").Replace(&[]models.Role{role}); err != nil {
		t.Fatal(err)
	}
	result, err := NewAuthService(db).Login(user.Username, "pass")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Permissions, ",") != "admin,read" {
		t.Fatalf("incorrect permissions: %v", result.Permissions)
	}
}

func TestPasswordVerifierRejectsCorruptAndUnboundedHashes(t *testing.T) {
	for _, hash := range []string{
		"plain-password",
		"$argon2id$v=19$m=4294967295,t=3,p=2$YWJj$YWJj",
		"$argon2id$v=19$m=65536,t=3,p=2$!!!!$!!!!",
		strings.Repeat("x", 300),
	} {
		if valid, _ := verifyPassword(hash, "pass"); valid {
			t.Fatal("invalid stored hash was accepted")
		}
	}
}

func TestAuthService_MigratesLegacyLongPasswords(t *testing.T) {
	db := newAuthTestDB(t)
	password := strings.Repeat("p", 100)
	user := seedAuthUser(t, db, "long-password", password, "long-uuid", "active", "")
	if _, err := NewAuthService(db).Login(user.Username, password); err != nil {
		t.Fatal(err)
	}
	var saved models.User
	db.First(&saved, user.Id)
	if !strings.HasPrefix(saved.Password, "$argon2id$") {
		t.Fatal("legacy long password was not migrated")
	}
}
