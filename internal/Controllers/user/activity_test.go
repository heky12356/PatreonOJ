package user

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"patreonoj/internal/models"
	submissionsvc "patreonoj/internal/services/submission"
	usersvc "patreonoj/internal/services/user"
	"strings"
	"testing"
)

func activityDB(t *testing.T) (*gorm.DB, models.User, models.User, models.Question) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Question{}, &models.Submission{}, &models.TestCase{}); err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { conn.Close() })
	alice := models.User{Username: "alice", UUID: "alice-uuid", Password: "hash", Status: "active"}
	bob := models.User{Username: "bob", UUID: "bob-uuid", Password: "hash", Status: "active"}
	q := models.Question{QuestionId: "1000", Title: "A + B", Status: "published"}
	for _, v := range []interface{}{&alice, &bob, &q} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, row := range []struct{ user, verdict string }{{alice.UUID, "AC"}, {alice.UUID, "WA"}, {bob.UUID, "WA"}} {
		sub := models.NewSubmission(row.user, q.Id, "print(3)", "python")
		sub.ID = fmt.Sprintf("submission-%d", i)
		sub.Status = "completed"
		sub.Verdict = row.verdict
		if err := db.Create(sub).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, alice, bob, q
}

func TestProblemStatusesUseSessionAndNeverAnotherAccount(t *testing.T) {
	db, alice, bob, q := activityDB(t)
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		viewer models.User
		want   string
	}{{alice, "solved"}, {bob, "attempted"}} {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("currentUser", tc.viewer) })
		r.GET("/status", NewUserController(db).ProblemStatuses)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/status?ids=%d&uuid=%s", q.Id, alice.UUID), nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) || !strings.Contains(w.Body.String(), tc.viewer.UUID) {
			t.Fatalf("wrong user state: %s", w.Body)
		}
	}
	stranger := models.User{Username: "new", UUID: "new-uuid", Password: "hash"}
	db.Create(&stranger)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("currentUser", stranger) })
	r.GET("/status", NewUserController(db).ProblemStatuses)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/status?ids=%d", q.Id), nil))
	if !strings.Contains(w.Body.String(), "unsolved") {
		t.Fatal(w.Body.String())
	}
}

func TestHomePrivacyProtectsListsDetailsAndLegacySolves(t *testing.T) {
	db, alice, bob, q := activityDB(t)
	service := usersvc.NewUserService(db)
	home, err := service.GetHome(alice.UUID, models.User{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !home.ShowActivity || !home.ActivityVisible || home.Stats.SolvedCount != 1 || len(home.Problems) != 1 || len(home.RecentSubmissions) != 2 {
		t.Fatalf("bad public home: %+v", home)
	}
	hidden := false
	if _, err := service.UpdateByID(fmt.Sprint(alice.Id), usersvc.UserUpdateInput{ShowActivity: &hidden}); err != nil {
		t.Fatal(err)
	}
	home, err = service.GetHome(alice.UUID, bob, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(home)
	if home.ActivityVisible || strings.Contains(string(data), "\"stats\"") || strings.Contains(string(data), "submission-") {
		t.Fatal(string(data))
	}
	own, err := service.GetHome(alice.UUID, alice, 1, 20)
	if err != nil || own.Stats == nil || own.Stats.SolvedCount != 1 {
		t.Fatalf("owner lost access: %v %+v", err, own)
	}
	list, err := submissionsvc.NewSubmissionQueryService(db).ListProblemSubmissionsForViewer(q.Id, "", "", 1, 20, bob.UUID, false)
	if err != nil || len(list.Items) != 1 || list.Items[0].Username != "bob" {
		t.Fatalf("private submissions leaked: %v %+v", err, list)
	}
	_, err = submissionsvc.NewSubmissionCommandService(db, nil).BuildSubmissionResultForViewer("submission-0", bob)
	if err != submissionsvc.ErrSubmissionForbidden {
		t.Fatalf("private detail accessible: %v", err)
	}
	result, err := submissionsvc.NewSubmissionCommandService(db, nil).BuildSubmissionResultForViewer("submission-0", alice)
	if err != nil || result["source_code"] != "print(3)" || result["verdict"] != "AC" || result["username"] != "alice" {
		t.Fatalf("bad own code view: %v %+v", err, result)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("currentUser", bob) })
	r.GET("/solves/:uuid", NewUserSolveController(db).Index)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/solves/"+alice.UUID, nil))
	if w.Code != 403 {
		t.Fatalf("legacy privacy bypass: %d %s", w.Code, w.Body)
	}
}

func TestVerdictFiltersApplyBeforePagination(t *testing.T) {
	db, alice, _, q := activityDB(t)
	list, err := submissionsvc.NewSubmissionQueryService(db).ListProblemSubmissionsForViewer(q.Id, "AC", "", 1, 1, alice.UUID, false)
	if err != nil || list.Total != 1 || len(list.Items) != 1 || list.Items[0].Verdict != "AC" {
		t.Fatalf("bad AC filter: %v %+v", err, list)
	}
	list, err = submissionsvc.NewSubmissionQueryService(db).ListProblemSubmissionsForViewer(q.Id, "WA", "", 1, 1, alice.UUID, false)
	if err != nil || list.Total != 2 || len(list.Items) != 1 || list.Items[0].Verdict != "WA" {
		t.Fatalf("bad WA pagination: %v %+v", err, list)
	}
}
