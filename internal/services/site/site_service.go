package site

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"patreonoj/internal/models"
)

var ErrInvalidSettings = errors.New("invalid site settings")
var themePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type Slide struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	ImageURL string `json:"image_url"`
	LinkURL  string `json:"link_url"`
}

type Carousel struct {
	Enabled    bool    `json:"enabled"`
	IntervalMS int     `json:"interval_ms"`
	Slides     []Slide `json:"slides"`
}

type Settings struct {
	ThemeID  string   `json:"theme_id"`
	Carousel Carousel `json:"carousel"`
}

type Update struct {
	ThemeID  *string   `json:"theme_id"`
	Carousel *Carousel `json:"carousel"`
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func defaults() models.SiteSettings {
	return models.SiteSettings{ID: 1, ThemeID: "default", CarouselIntervalMS: 6000, SlidesJSON: "[]"}
}

func decode(row models.SiteSettings) (Settings, error) {
	result := Settings{ThemeID: row.ThemeID, Carousel: Carousel{Enabled: row.CarouselEnabled, IntervalMS: row.CarouselIntervalMS, Slides: []Slide{}}}
	err := json.Unmarshal([]byte(row.SlidesJSON), &result.Carousel.Slides)
	if result.Carousel.Slides == nil {
		result.Carousel.Slides = []Slide{}
	}
	return result, err
}

func (s *Service) Get() (Settings, error) {
	var row models.SiteSettings
	err := s.db.First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = defaults()
	} else if err != nil {
		return Settings{}, err
	}
	return decode(row)
}

// URLs are used only by browsers. Relative links must stay on this origin.
func SafeURL(raw string, required bool) bool {
	if raw == "" {
		return !required
	}
	if len(raw) > 2048 || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\\\r\n\t") || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.ContainsAny(decoded, "\\\r\n\t") || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
		return false
	}
	if strings.HasPrefix(raw, "/") {
		return !strings.HasPrefix(decoded, "//") && u.Host == "" && u.Scheme == ""
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != ""
}

func validate(update *Update) error {
	if update.ThemeID == nil && update.Carousel == nil {
		return ErrInvalidSettings
	}
	if update.ThemeID != nil && !themePattern.MatchString(*update.ThemeID) {
		return ErrInvalidSettings
	}
	if c := update.Carousel; c != nil {
		if len(c.Slides) > 10 || c.IntervalMS < 3000 || c.IntervalMS > 30000 || (c.Enabled && len(c.Slides) == 0) {
			return ErrInvalidSettings
		}
		seen := map[string]bool{}
		for i := range c.Slides {
			slide := &c.Slides[i]
			if slide.ID == "" {
				slide.ID = uuid.NewString()
			}
			id, err := uuid.Parse(slide.ID)
			if err != nil || id.String() != slide.ID || seen[slide.ID] || len([]rune(slide.Title)) > 120 || !SafeURL(slide.ImageURL, true) || !SafeURL(slide.LinkURL, false) {
				return ErrInvalidSettings
			}
			seen[slide.ID] = true
		}
	}
	return nil
}

func (s *Service) Update(input Update) (Settings, error) {
	if err := validate(&input); err != nil {
		return Settings{}, err
	}
	var result Settings
	err := s.db.Transaction(func(tx *gorm.DB) error {
		row := defaults()
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, 1).Error; err != nil {
			return err
		}
		if input.ThemeID != nil {
			row.ThemeID = *input.ThemeID
		}
		if c := input.Carousel; c != nil {
			b, err := json.Marshal(c.Slides)
			if err != nil {
				return err
			}
			row.CarouselEnabled = c.Enabled
			row.CarouselIntervalMS = c.IntervalMS
			row.SlidesJSON = string(b)
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		var err error
		result, err = decode(row)
		return err
	})
	return result, err
}
