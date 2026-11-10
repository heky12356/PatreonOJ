package question

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestShowMissingQuestionDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Question{}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ctrl := NewQuestionController(db, nil)
	r.GET("/question/:id", ctrl.Show)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/question/12345", nil))
	if w.Code != 404 {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateExplicitEmptyTagsAndContent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Question{}, &models.QuestionRelationEdge{}); err != nil {
		t.Fatal(err)
	}
	q := models.Question{QuestionId: "q1", Title: "Title", Content: "original", Tags: "array", Status: "published"}
	if err = db.Create(&q).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ctrl := NewQuestionController(db, nil)
	r.POST("/question/:id", ctrl.Update)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/question/%d", q.Id), strings.NewReader(`{"tags":"","content":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
	if err = db.First(&q, q.Id).Error; err != nil {
		t.Fatal(err)
	}
	if q.Tags != "" || q.Content != "" || q.Title != "Title" {
		t.Fatalf("unexpected question: %+v", q)
	}
}
