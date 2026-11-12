package learning

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"patreonoj/internal/models"
	questionsvc "patreonoj/internal/services/question"
	"testing"
)

func TestGraphCacheSeparatesDraftAccessAndInvalidatesOnVisibilityChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := db.AutoMigrate(&models.Question{}, &models.QuestionRelationEdge{}); err != nil {
		t.Fatal(err)
	}
	q := models.Question{QuestionId: "private", Title: "Private", Status: "draft"}
	if err := db.Create(&q).Error; err != nil {
		t.Fatal(err)
	}
	controller := NewLearningController(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("Test-Admin") == "yes" {
			c.Set("currentUser", models.User{Id: 1, UUID: "admin", Permissions: "admin"})
		}
		c.Next()
	})
	router.GET("/graph/:id", controller.GetQuestionGraph)
	request := func(admin bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", fmt.Sprintf("/graph/%d", q.Id), nil)
		if admin {
			r.Header.Set("Test-Admin", "yes")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request(true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := request(false); w.Code != 404 {
		t.Fatal("draft response leaked from admin cache")
	}
	published := "published"
	svc := questionsvc.NewQuestionService(db)
	if _, err := svc.UpdateFieldsByID(q.Id, questionsvc.QuestionUpdateInput{Status: &published}); err != nil {
		t.Fatal(err)
	}
	if w := request(false); w.Code != 200 || w.Header().Get("X-Graph-Cache") != "MISS" {
		t.Fatalf("first public response: %d", w.Code)
	}
	if w := request(false); w.Code != 200 || w.Header().Get("X-Graph-Cache") != "HIT" {
		t.Fatal("response did not hit cache")
	}
	draft := "draft"
	if _, err := svc.UpdateFieldsByID(q.Id, questionsvc.QuestionUpdateInput{Status: &draft}); err != nil {
		t.Fatal(err)
	}
	if w := request(false); w.Code != 404 {
		t.Fatal("published cache survived hide operation")
	}
}
