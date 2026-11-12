// import_practice adds a private problem pack to an existing database without resetting it.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type testCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Hidden bool   `json:"hidden"`
}

type problem struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Difficulty  string     `json:"difficulty"`
	Tags        []string   `json:"tags"`
	TimeLimit   int        `json:"time_limit"`
	MemoryLimit int        `json:"memory_limit"`
	Cases       []testCase `json:"cases"`
}

type edge struct {
	From string                          `json:"from"`
	To   string                          `json:"to"`
	Type models.QuestionRelationEdgeType `json:"type"`
}

type pack struct {
	ID       string    `json:"id"`
	Problems []problem `json:"problems"`
	Edges    []edge    `json:"edges"`
}

type report struct {
	Pack       string         `json:"pack"`
	Apply      bool           `json:"applied"`
	Created    int            `json:"created"`
	Existing   int            `json:"existing"`
	TestCases  int            `json:"test_cases"`
	AddedEdges int            `json:"added_edges"`
	Questions  map[string]int `json:"questions"`
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func (p pack) validate() error {
	if !safeID.MatchString(p.ID) || len(p.Problems) == 0 || len(p.Problems) > 500 {
		return errors.New("pack requires a safe id and 1..500 problems")
	}
	ids := make(map[string]bool)
	for _, q := range p.Problems {
		if !safeID.MatchString(q.ID) || ids[q.ID] {
			return fmt.Errorf("invalid or duplicate problem id %q", q.ID)
		}
		ids[q.ID] = true
		if strings.TrimSpace(q.Title) == "" || strings.TrimSpace(q.Content) == "" || strings.TrimSpace(q.Difficulty) == "" {
			return fmt.Errorf("%s: title, content and difficulty are required", q.ID)
		}
		if q.TimeLimit < 100 || q.TimeLimit > 30000 || q.MemoryLimit < 16 || q.MemoryLimit > 1024 || len(q.Cases) < 2 || len(q.Cases) > 100 {
			return fmt.Errorf("%s: invalid limits or testcase count (2..100)", q.ID)
		}
		tags := make(map[string]bool)
		for _, tag := range q.Tags {
			if tag != strings.TrimSpace(tag) || tag == "" || len([]rune(tag)) > 64 || strings.Contains(tag, ",") || tags[tag] {
				return fmt.Errorf("%s: invalid tag", q.ID)
			}
			tags[tag] = true
		}
		if len(tags) == 0 {
			return fmt.Errorf("%s: tags are required", q.ID)
		}
		public, hidden := 0, 0
		for _, c := range q.Cases {
			if len(c.Input) > 16<<20 || len(c.Output) > 16<<20 || c.Output == "" {
				return fmt.Errorf("%s: invalid testcase size/output", q.ID)
			}
			if c.Hidden {
				hidden++
			} else {
				public++
			}
		}
		if public == 0 || hidden == 0 {
			return fmt.Errorf("%s: both public and hidden cases are required", q.ID)
		}
	}
	seen := make(map[edge]bool)
	for _, e := range p.Edges {
		if !safeID.MatchString(e.From) || !safeID.MatchString(e.To) || e.From == e.To || !e.Type.Valid() || seen[e] {
			return fmt.Errorf("invalid or duplicate edge: %+v", e)
		}
		seen[e] = true
	}
	return nil
}

func row(q problem) models.Question {
	return models.Question{QuestionId: q.ID, Title: q.Title, Content: q.Content, Difficulty: q.Difficulty,
		Tags: strings.Join(q.Tags, ","), TimeLimit: q.TimeLimit, MemoryLimit: q.MemoryLimit, Status: "published"}
}

func cases(p pack, q problem, pk int) []models.TestCase {
	b, _ := json.Marshal(q)
	hash := sha256.Sum256(b)
	prefix := "testcases/import/" + p.ID + "/" + q.ID + "/" + hex.EncodeToString(hash[:16])
	out := make([]models.TestCase, len(q.Cases))
	for i, c := range q.Cases {
		key := fmt.Sprintf("%s/%03d", prefix, i+1)
		out[i] = models.TestCase{QuestionID: pk, InputKey: key + ".in", OutputKey: key + ".out", InputSize: int64(len(c.Input)), OutputSize: int64(len(c.Output)), IsHidden: c.Hidden}
	}
	return out
}

func sameQuestion(a, b models.Question) bool {
	return a.QuestionId == b.QuestionId && a.Title == b.Title && a.Content == b.Content && a.Difficulty == b.Difficulty && a.Tags == b.Tags && a.TimeLimit == b.TimeLimit && a.MemoryLimit == b.MemoryLimit && a.Status == b.Status
}

// preflight rejects collisions before any objects are uploaded. Existing rows are never updated.
func preflight(db *gorm.DB, p pack) (map[string]int, error) {
	ids := make(map[string]int)
	for _, q := range p.Problems {
		var found []models.Question
		if err := db.Where("question_id = ?", q.ID).Find(&found).Error; err != nil {
			return nil, err
		}
		if len(found) == 0 {
			ids[q.ID] = 0
			continue
		}
		if len(found) != 1 || !sameQuestion(found[0], row(q)) {
			return nil, fmt.Errorf("%s already exists with different content; choose a new id", q.ID)
		}
		var stored []models.TestCase
		if err := db.Where("question_id = ?", found[0].Id).Order("id").Find(&stored).Error; err != nil {
			return nil, err
		}
		expected := cases(p, q, found[0].Id)
		if len(stored) != len(expected) {
			return nil, fmt.Errorf("%s: existing testcase count differs", q.ID)
		}
		for i := range stored {
			stored[i].ID = 0
			if stored[i] != expected[i] {
				return nil, fmt.Errorf("%s: existing testcases differ", q.ID)
			}
		}
		ids[q.ID] = found[0].Id
	}
	for _, e := range p.Edges {
		for _, id := range []string{e.From, e.To} {
			if _, ok := ids[id]; ok {
				continue
			}
			var found []models.Question
			if err := db.Where("question_id = ?", id).Find(&found).Error; err != nil {
				return nil, err
			}
			if len(found) != 1 || found[0].Status != "published" {
				return nil, fmt.Errorf("edge endpoint %s must resolve to one published question", id)
			}
			ids[id] = found[0].Id
		}
	}
	return ids, nil
}

type putObject func(context.Context, string, string) error

func importPack(ctx context.Context, db *gorm.DB, p pack, apply bool, put putObject) (report, error) {
	r := report{Pack: p.ID, Apply: apply}
	if err := p.validate(); err != nil {
		return r, err
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids, err := preflight(tx, p)
		if err != nil {
			return err
		}
		r.Questions = ids
		for _, q := range p.Problems {
			if ids[q.ID] != 0 {
				r.Existing++
				continue
			}
			r.Created++
			r.TestCases += len(q.Cases)
			if !apply {
				continue
			}
			if put == nil {
				return errors.New("object uploader is required")
			}
			qr := row(q)
			if err := tx.Create(&qr).Error; err != nil {
				return err
			}
			ids[q.ID] = qr.Id
			cs := cases(p, q, qr.Id)
			for i, c := range cs {
				if err := put(ctx, c.InputKey, q.Cases[i].Input); err != nil {
					return fmt.Errorf("%s input upload: %w", q.ID, err)
				}
				if err := put(ctx, c.OutputKey, q.Cases[i].Output); err != nil {
					return fmt.Errorf("%s output upload: %w", q.ID, err)
				}
			}
			if err := tx.Create(&cs).Error; err != nil {
				return err
			}
			for _, name := range q.Tags {
				tag := models.Tag{Name: name}
				if err := tx.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
					return err
				}
				link := models.QuestionTag{QuestionId: qr.Id, TagId: tag.ID}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
					return err
				}
			}
		}
		for _, e := range p.Edges {
			if !apply {
				continue
			}
			rel := models.QuestionRelationEdge{FromQuestionID: e.From, ToQuestionID: e.To, EdgeType: e.Type}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
			if result.Error != nil {
				return result.Error
			}
			r.AddedEdges += int(result.RowsAffected)
		}
		return nil
	})
	return r, err
}

func run() error {
	file := flag.String("file", "", "private JSON problem pack (contains hidden test data)")
	configFile := flag.String("config", "config.yaml", "existing application config")
	apply := flag.Bool("apply", false, "write problems, testcases and explicit edges; default is database preflight only")
	flag.Parse()
	if *file == "" {
		return errors.New("usage: import_practice -file pack.json [-config config.yaml] [-apply]")
	}
	f, err := os.Open(*file)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 64<<20))
	decoder.DisallowUnknownFields()
	var p pack
	if err := decoder.Decode(&p); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing JSON or oversized pack")
	}
	if err := p.validate(); err != nil {
		return err
	}
	if err := config.InitConfig(*configFile); err != nil {
		return err
	}
	if err := models.InitDB(); err != nil {
		return err
	}
	sqlDB, err := models.DB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var put putObject
	if *apply {
		cfg := config.GlobalConfig.OSS
		client, err := oss.NewOSSClient(cfg.Address, cfg.PublicAddress, cfg.AccessKey, cfg.SecretKey)
		if err != nil {
			return err
		}
		put = func(ctx context.Context, key, body string) error {
			_, err := client.UploadFile(ctx, cfg.BucketName, key, bytes.NewBufferString(body), int64(len(body)), "text/plain; charset=utf-8")
			return err
		}
	}
	r, err := importPack(ctx, models.DB, p, *apply, put)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(r)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
