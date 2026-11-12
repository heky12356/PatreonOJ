package site

import (
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"patreonoj/internal/models"
	"testing"
)

func database(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.SiteSettings{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { sql.Close() })
	return db
}

func TestSettingsPersistAndIndependentUpdates(t *testing.T) {
	db := database(t)
	s := NewService(db)
	initial, err := s.Get()
	if err != nil || initial.ThemeID != "default" || initial.Carousel.Enabled || initial.Carousel.IntervalMS != 6000 || initial.Carousel.Slides == nil {
		t.Fatalf("defaults: %+v %v", initial, err)
	}
	var count int64
	db.Model(&models.SiteSettings{}).Count(&count)
	if count != 0 {
		t.Fatal("public read created database rows")
	}
	c := Carousel{Enabled: true, IntervalMS: 5000, Slides: []Slide{{Title: "Course", ImageURL: "/api/site/images/example.png", LinkURL: "/problem"}}}
	withImages, err := s.Update(Update{Carousel: &c})
	if err != nil || len(withImages.Carousel.Slides) != 1 || withImages.Carousel.Slides[0].ID == "" {
		t.Fatalf("save images: %+v %v", withImages, err)
	}
	theme := "local-campus"
	result, err := s.Update(Update{ThemeID: &theme})
	if err != nil || result.Carousel.Slides[0] != withImages.Carousel.Slides[0] || !result.Carousel.Enabled {
		t.Fatalf("theme changed carousel: %+v %v", result, err)
	}
	c.Enabled = false
	result, err = s.Update(Update{Carousel: &c})
	if err != nil || result.ThemeID != theme || result.Carousel.Enabled || len(result.Carousel.Slides) != 1 {
		t.Fatalf("disabling lost state: %+v %v", result, err)
	}
	fresh, err := NewService(db).Get()
	if err != nil || fresh.ThemeID != theme || fresh.Carousel.Slides[0].Title != "Course" {
		t.Fatal("settings did not persist")
	}
}

func TestSettingsRejectUnsafeInputWithoutChangingState(t *testing.T) {
	s := NewService(database(t))
	badTheme := "../../theme"
	if _, err := s.Update(Update{ThemeID: &badTheme}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatal("unsafe theme accepted")
	}
	for _, bad := range []Carousel{
		{Enabled: true, IntervalMS: 6000, Slides: []Slide{}},
		{IntervalMS: 1000},
		{IntervalMS: 6000, Slides: []Slide{{ImageURL: "javascript:alert(1)"}}},
		{IntervalMS: 6000, Slides: []Slide{{ImageURL: "https://example.com/a.png", LinkURL: "//evil.example/path"}}},
	} {
		if _, err := s.Update(Update{Carousel: &bad}); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("accepted %+v", bad)
		}
	}
	state, err := s.Get()
	if err != nil || state.ThemeID != "default" || len(state.Carousel.Slides) != 0 {
		t.Fatal("invalid write changed settings")
	}
}

func TestSafeURL(t *testing.T) {
	for _, v := range []string{"/problem", "/api/site/images/a.png", "https://example.com/a.png?x=1", "http://example.com"} {
		if !SafeURL(v, true) {
			t.Errorf("rejected %q", v)
		}
	}
	for _, v := range []string{"", "javascript:alert(1)", "data:image/svg+xml,hi", "//evil.example", "/%2Fevil.example", "/\\evil.example", "/a%0d%0aheader", "https://user:pass@example.com/a", "file:///tmp/a", " /problem"} {
		if SafeURL(v, true) {
			t.Errorf("accepted %q", v)
		}
	}
}
