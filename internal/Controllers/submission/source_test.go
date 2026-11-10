package submission

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"patreonoj/internal/models"
	submissionsvc "patreonoj/internal/services/submission"
	"testing"
)

func TestOwnSourceUsesAnEnvelopeCompatibleWithOlderClients(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if err := db.AutoMigrate(&models.User{}, &models.Question{}, &models.Submission{}); err != nil {
		t.Fatal(err)
	}
	owner := models.User{Username: "owner", UUID: "owner-uuid", Password: "hash", Status: "active"}
	db.Create(&owner)
	db.Model(&owner).Update("show_activity", false)
	question := models.Question{QuestionId: "1000", Title: "A+B", Status: "published"}
	db.Create(&question)
	source := "#include <iostream>\nint main(){ std::cout << 3; }\n"
	record := models.NewSubmission(owner.UUID, question.Id, source, "cpp")
	db.Create(record)
	controller := &SubmissionController{commandService: submissionsvc.NewSubmissionCommandService(db, nil)}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("currentUser", owner) })
	r.GET("/submission/:id", controller.GetSubmissionResult)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/submission/"+record.ID, nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != float64(200) {
		t.Fatal("numeric API status code was overwritten")
	}
	detail, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data envelope")
	}
	if detail["source_code"] != source || detail["code"] != source || detail["language"] != "cpp" {
		t.Fatal("owner or legacy client cannot read source")
	}
}
