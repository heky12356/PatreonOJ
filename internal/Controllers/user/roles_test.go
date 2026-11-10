package user

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"patreonoj/internal/models"
	"patreonoj/internal/util"
	"strings"
	"testing"
)

func TestAdminCanGrantAndRemoveAnInitializedRole(t *testing.T) {
	db, admin, target, _ := activityDB(t)
	if err := db.AutoMigrate(&models.Role{}, &models.Permission{}); err != nil {
		t.Fatal(err)
	}
	if err := models.EnsureBuiltinRoles(db); err != nil {
		t.Fatal(err)
	}
	admin.Permissions = "admin"
	db.Model(&admin).Update("permissions", "admin")
	router := func(viewer models.User) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("currentUser", viewer) })
		controller := NewUserController(db)
		r.GET("/user/roles", controller.Roles)
		r.PUT("/user/:id", controller.Update)
		return r
	}
	r := router(admin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/user/roles", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "\"name\":\"admin\"") {
		t.Fatal(w.Code, w.Body)
	}
	for _, roles := range []string{`["admin"]`, `[]`} {
		body := fmt.Sprintf(`{"permissions":"","role_codes":%s}`, roles)
		req := httptest.NewRequest("PUT", "/user/"+target.UUID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var saved models.User
		db.Preload("Roles.Permissions").First(&saved, target.Id)
		if util.UserInstance.HasPermission(saved, "admin") != (roles != `[]`) {
			t.Fatal("role did not update effective access")
		}
		if saved.Permissions != "" {
			t.Fatal("inherited permission became a direct grant")
		}
	}
	w = httptest.NewRecorder()
	router(target).ServeHTTP(w, httptest.NewRequest("GET", "/user/roles", nil))
	if w.Code != 403 {
		t.Fatal("ordinary user can read administration roles")
	}
	req := httptest.NewRequest("PUT", "/user/"+target.UUID, strings.NewReader(`{"role_codes":["admin"]}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router(target).ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("ordinary user can grant roles")
	}
}
