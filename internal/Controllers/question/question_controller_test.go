package question

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIndexExactTagFilterAndInputLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Question{}); err != nil {
		t.Fatal(err)
	}
	rows := []models.Question{
		{QuestionId: "A", Title: "sum", Tags: "数组,前缀和", Difficulty: "简单", Status: "published"},
		{QuestionId: "B", Title: "sum", Tags: "二维数组", Difficulty: "简单", Status: "published"},
		{QuestionId: "C", Title: "sum", Tags: "数组", Difficulty: "简单", Status: "draft"},
	}
	if err = db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/question/", NewQuestionController(db, nil).Index)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/question/?"+url.Values{"tag": {"数组"}, "q": {"sum"}, "difficult": {"简单"}}.Encode(), nil))
	var result struct {
		Total int               `json:"totalCnt"`
		Items []models.Question `json:"result"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result.Total != 1 || len(result.Items) != 1 || result.Items[0].QuestionId != "A" {
		t.Fatalf("unexpected public filtered response: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/question/?tag="+url.QueryEscape(strings.Repeat("数", 129)), nil))
	if w.Code != 400 {
		t.Fatalf("oversized tag: %d", w.Code)
	}
}

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
