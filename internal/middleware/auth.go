package middleware

import (
	"errors"
	"net/http"
	"time"

	"patreonoj/internal/models"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func LoadCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			c.Next()
			return
		}

		if sessionID == "" {
			c.Next()
			return
		}

		var session models.Session
		if err := db.WithContext(c.Request.Context()).Where("session_id = ?", sessionID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Set("authError", "invalid_session")
			} else {
				c.Set("authError", "authentication_unavailable")
				c.Set("authStatus", http.StatusServiceUnavailable)
			}
			c.Next()
			return
		}

		if session.RevokedAt != nil || session.ExpiresAt == nil || !session.ExpiresAt.After(time.Now()) {
			c.Set("authError", "expired_session")
			c.Next()
			return
		}

		var user models.User
		if err := db.WithContext(c.Request.Context()).Preload("Roles.Permissions").First(&user, session.UserId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Set("authError", "user_not_found")
			} else {
				c.Set("authError", "authentication_unavailable")
				c.Set("authStatus", http.StatusServiceUnavailable)
			}
			c.Next()
			return
		}
		if user.Status != "" && user.Status != "active" {
			c.Set("authError", "user_disabled")
			c.Set("authStatus", http.StatusForbidden)
			c.Next()
			return
		}

		c.Set("currentUser", user)
		c.Set("sessionId", sessionID)
		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if value, ok := c.Get("currentUser"); ok {
			if user, valid := value.(models.User); valid && user.Id > 0 && (user.Status == "" || user.Status == "active") {
				c.Next()
				return
			}
		}
		abortAuthentication(c)
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get("currentUser")
		user, valid := value.(models.User)
		if !ok || !valid || user.Id <= 0 {
			abortAuthentication(c)
			return
		}

		if !util.UserInstance.HasPermission(user, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "permission denied", "error": "permission denied"})
			return
		}

		c.Next()
	}
}

func abortAuthentication(c *gin.Context) {
	status := http.StatusUnauthorized
	if value, ok := c.Get("authStatus"); ok {
		if authStatus, valid := value.(int); valid {
			status = authStatus
		}
	}
	message := "unauthorized"
	if value, ok := c.Get("authError"); ok {
		if authError, valid := value.(string); valid {
			message = authError
		}
	}
	c.AbortWithStatusJSON(status, gin.H{"code": status, "msg": message, "error": message})
}
