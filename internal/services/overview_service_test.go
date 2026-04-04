package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOverviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.OjOverView{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedOverview(t *testing.T, db *gorm.DB, homeText, announcement string) {
	t.Helper()
	err := db.Create(&models.OjOverView{
		HomeText:     homeText,
		Announcement: announcement,
	}).Error
	if err != nil {
		t.Fatalf("seed overview failed: %v", err)
	}
}

func TestOverviewService_GetHomeText_NotFound(t *testing.T) {
	db := newOverviewTestDB(t)
	svc := NewOverviewService(db)

	_, err := svc.GetHomeText()
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestOverviewService_HomeTextFlow(t *testing.T) {
	db := newOverviewTestDB(t)
	seedOverview(t, db, "old", "ann")
	svc := NewOverviewService(db)

	homeText, err := svc.GetHomeText()
	if err != nil {
		t.Fatalf("GetHomeText failed: %v", err)
	}
	if homeText != "old" {
		t.Fatalf("expected old, got %s", homeText)
	}

	updated, err := svc.UpdateHomeText("new")
	if err != nil {
		t.Fatalf("UpdateHomeText failed: %v", err)
	}
	if updated != "new" {
		t.Fatalf("expected new, got %s", updated)
	}

	check, err := svc.GetHomeText()
	if err != nil {
		t.Fatalf("GetHomeText after update failed: %v", err)
	}
	if check != "new" {
		t.Fatalf("expected new after update, got %s", check)
	}
}

func TestOverviewService_AnnouncementFlow(t *testing.T) {
	db := newOverviewTestDB(t)
	seedOverview(t, db, "home", "old-ann")
	svc := NewOverviewService(db)

	announcement, err := svc.GetAnnouncement()
	if err != nil {
		t.Fatalf("GetAnnouncement failed: %v", err)
	}
	if announcement != "old-ann" {
		t.Fatalf("expected old-ann, got %s", announcement)
	}

	updated, err := svc.UpdateAnnouncement("new-ann")
	if err != nil {
		t.Fatalf("UpdateAnnouncement failed: %v", err)
	}
	if updated != "new-ann" {
		t.Fatalf("expected new-ann, got %s", updated)
	}

	check, err := svc.GetAnnouncement()
	if err != nil {
		t.Fatalf("GetAnnouncement after update failed: %v", err)
	}
	if check != "new-ann" {
		t.Fatalf("expected new-ann after update, got %s", check)
	}
}
