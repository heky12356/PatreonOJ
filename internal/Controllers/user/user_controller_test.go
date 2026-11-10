package user

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUpdateUsesURLIdentityAndPreservesPermissions(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"body-id-optional", `{"nickname":"updated"}`, http.StatusOK},
		{"conflicting-body-id", `{"id":"9999","nickname":"updated"}`, http.StatusBadRequest},
		{"invalid-body-id", `{"id":"bad","nickname":"updated"}`, http.StatusBadRequest},
		{"cannot-grant-admin", `{"permissions":"admin"}`, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&models.User{}); err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			t.Cleanup(func() { _ = sqlDB.Close() })
			user := models.User{Username: "ordinary", Password: "unused", UUID: "ordinary-uuid", Status: "active"}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("currentUser", user); c.Next() })
			r.PUT("/user/update/:id", NewUserController(db).Update)
			req := httptest.NewRequest(http.MethodPut, "/user/update/"+strconv.Itoa(user.Id), strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d body=%s", tc.want, w.Code, w.Body)
			}
			var saved models.User
			if err := db.First(&saved, user.Id).Error; err != nil {
				t.Fatal(err)
			}
			if saved.Permissions != "" {
				t.Fatal("ordinary user escalated permissions")
			}
			if tc.want == http.StatusOK && saved.Nickname != "updated" {
				t.Fatal("nickname update was not saved")
			}
			if tc.want != http.StatusOK && saved.Nickname != "" {
				t.Fatal("rejected update modified the user")
			}
		})
	}
}
