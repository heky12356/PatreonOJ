package auth

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"patreonoj/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAuthInvalidCredentials = errors.New("invalid credentials")
	ErrAuthUserDisabled       = errors.New("user disabled")
	ErrAuthUsernameExists     = errors.New("username already exists")
	ErrAuthUserNotFound       = errors.New("user not found")
	ErrAuthSessionNotFound    = errors.New("session not found")
	ErrAuthInvalidInput       = errors.New("username must contain 1 to 64 characters and password must contain 1 to 4096 bytes")
	ErrAuthUUIDGeneration     = errors.New("unable to generate unique uuid")
)

type AuthService struct {
	db                 *gorm.DB
	uuidGenerator      func() string
	sessionIDGenerator func() (string, error)
}

type AuthLoginResult struct {
	UserID      int
	UUID        string
	Username    string
	Permissions []string
	SessionID   string
	ExpiresAt   time.Time
}

type AuthRegisterResult struct {
	UUID   string
	UserID int
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		db:                 db,
		uuidGenerator:      func() string { return uuid.New().String() },
		sessionIDGenerator: NewSessionID,
	}
}

func validateCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" || utf8.RuneCountInString(username) > 64 || len(password) == 0 || len(password) > 4096 {
		return ErrAuthInvalidInput
	}
	return nil
}

func (s *AuthService) Login(username, password string) (*AuthLoginResult, error) {
	username = strings.TrimSpace(username)
	if err := validateCredentials(username, password); err != nil {
		return nil, err
	}
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthInvalidCredentials
		}
		return nil, err
	}
	valid, legacy := verifyPassword(user.Password, password)
	if !valid {
		return nil, ErrAuthInvalidCredentials
	}
	if user.Status != "" && user.Status != "active" {
		return nil, ErrAuthUserDisabled
	}
	var upgradedHash string
	if legacy {
		var err error
		upgradedHash, err = HashPassword(password)
		if err != nil {
			return nil, err
		}
	}
	sessionID, err := s.sessionIDGenerator()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, user.Id).Error; err != nil {
			return err
		}
		if current.Status != "" && current.Status != "active" {
			return ErrAuthUserDisabled
		}
		if current.Password != user.Password {
			if valid, _ := verifyPassword(current.Password, password); !valid {
				return ErrAuthInvalidCredentials
			}
		}
		if upgradedHash != "" && current.Password == user.Password {
			if err := tx.Model(&current).Update("password", upgradedHash).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.Session{SessionId: sessionID, UserId: user.Id, ExpiresAt: &expiresAt}).Error
	})
	if err != nil {
		return nil, err
	}
	return &AuthLoginResult{
		UserID: user.Id, UUID: user.UUID, Username: user.Username,
		Permissions: permissionCodes(user), SessionID: sessionID, ExpiresAt: expiresAt,
	}, nil
}

func permissionCodes(user models.User) []string {
	seen := make(map[string]struct{})
	for _, code := range strings.Split(user.Permissions, ",") {
		if code = strings.TrimSpace(code); code != "" {
			seen[code] = struct{}{}
		}
	}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			if code := strings.TrimSpace(permission.Code); code != "" {
				seen[code] = struct{}{}
			}
		}
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func (s *AuthService) Register(username, password string) (*AuthRegisterResult, error) {
	username = strings.TrimSpace(username)
	if err := validateCredentials(username, password); err != nil {
		return nil, err
	}
	var existing models.User
	if err := s.db.Where("username = ?", username).First(&existing).Error; err == nil {
		return nil, ErrAuthUsernameExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	userUUID, err := s.generateUniqueUUID()
	if err != nil {
		return nil, err
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := models.User{Username: username, Password: passwordHash, UUID: userUUID}
	if err := s.db.Create(&user).Error; err != nil {
		// Resolve a concurrent registration of the same username consistently.
		if s.db.Where("username = ?", username).First(&existing).Error == nil {
			return nil, ErrAuthUsernameExists
		}
		return nil, err
	}
	return &AuthRegisterResult{UUID: userUUID, UserID: user.Id}, nil
}

func (s *AuthService) Logout(sessionID string) error {
	var session models.Session
	if err := s.db.Where("session_id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAuthSessionNotFound
		}
		return err
	}
	if session.RevokedAt != nil {
		return nil
	}
	return s.db.Model(&session).Update("revoked_at", time.Now()).Error
}

func (s *AuthService) generateUniqueUUID() (string, error) {
	for i := 0; i < 10; i++ {
		userUUID := s.uuidGenerator()
		var existing models.User
		if err := s.db.Where("uuid = ?", userUUID).First(&existing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return userUUID, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", ErrAuthUUIDGeneration
}

// md5EncodeAuth is retained solely for verifying and migrating legacy accounts.
func md5EncodeAuth(value string) string {
	digest := md5.Sum([]byte(value))
	return hex.EncodeToString(digest[:])
}

func legacyPasswordMatches(hash, password string) bool {
	if len(hash) != 32 {
		return false
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(hash)), []byte(md5EncodeAuth(password))) == 1
}

func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
