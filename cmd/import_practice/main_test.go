package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"patreonoj/internal/models"
)

func fixture(t *testing.T) (*gorm.DB, pack) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.TestCase{}, &models.Tag{}, &models.QuestionTag{}, &models.QuestionRelationEdge{}); err != nil {
		t.Fatal(err)
	}
	q := models.Question{QuestionId: "1000", Title: "existing", Status: "published"}
	if err := db.Create(&q).Error; err != nil {
		t.Fatal(err)
	}
	p := pack{ID: "practice-v1", Problems: []problem{{ID: "KG1", Title: "new", Content: "statement", Difficulty: "简单", Tags: []string{"数组"}, TimeLimit: 2000, MemoryLimit: 256, Cases: []testCase{{Input: "1\n", Output: "2\n"}, {Input: "3\n", Output: "4\n", Hidden: true}}}}, Edges: []edge{{From: "1000", To: "KG1", Type: models.QuestionRelationEdgeTypeNextLevel}}}
	return db, p
}

func count(t *testing.T, db *gorm.DB, model interface{}) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDryRunAndIdempotentAppend(t *testing.T) {
	db, p := fixture(t)
	ctx := context.Background()
	uploads := 0
	put := func(context.Context, string, string) error { uploads++; return nil }
	r, err := importPack(ctx, db, p, false, put)
	if err != nil || r.Created != 1 || uploads != 0 || count(t, db, &models.Question{}) != 1 {
		t.Fatalf("dry run wrote data: %+v %v", r, err)
	}
	r, err = importPack(ctx, db, p, true, put)
	if err != nil || r.Created != 1 || r.AddedEdges != 1 || uploads != 4 {
		t.Fatalf("import: %+v %v uploads=%d", r, err, uploads)
	}
	r, err = importPack(ctx, db, p, true, put)
	if err != nil || r.Created != 0 || r.Existing != 1 || r.AddedEdges != 0 || uploads != 4 {
		t.Fatalf("repeat changed data: %+v %v", r, err)
	}
	if count(t, db, &models.Question{}) != 2 || count(t, db, &models.TestCase{}) != 2 || count(t, db, &models.QuestionTag{}) != 1 {
		t.Fatal("unexpected row counts")
	}
	var old models.Question
	db.Where("question_id = ?", "1000").First(&old)
	if old.Title != "existing" {
		t.Fatal("existing problem was modified")
	}
}

func TestCollisionsAndMissingEndpointRejectedBeforeUpload(t *testing.T) {
	for _, scenario := range []string{"different-content", "different-cases", "unknown-endpoint", "duplicate-id"} {
		t.Run(scenario, func(t *testing.T) {
			db, p := fixture(t)
			ctx := context.Background()
			put := func(context.Context, string, string) error { return nil }
			if scenario == "different-content" || scenario == "different-cases" {
				if _, err := importPack(ctx, db, p, true, put); err != nil {
					t.Fatal(err)
				}
				if scenario == "different-content" {
					p.Problems[0].Title = "changed"
				} else {
					p.Problems[0].Cases[0].Output = "different"
				}
			} else if scenario == "unknown-endpoint" {
				p.Edges[0].From = "missing"
			} else {
				p.Problems = append(p.Problems, p.Problems[0])
			}
			uploads := 0
			_, err := importPack(ctx, db, p, true, func(context.Context, string, string) error { uploads++; return nil })
			if err == nil || uploads != 0 {
				t.Fatalf("expected preflight rejection, got %v uploads=%d", err, uploads)
			}
		})
	}
}

func TestUploadFailureRollsBackDatabase(t *testing.T) {
	db, p := fixture(t)
	_, err := importPack(context.Background(), db, p, true, func(context.Context, string, string) error { return errors.New("object store unavailable") })
	if err == nil {
		t.Fatal("expected failure")
	}
	if count(t, db, &models.Question{}) != 1 || count(t, db, &models.TestCase{}) != 0 || count(t, db, &models.QuestionRelationEdge{}) != 0 {
		t.Fatal("failed import left database rows")
	}
}
