package services

import (
	"context"
	"errors"
	"testing"

	"patreonoj/internal/graph"
)

type mockUserMasteryReader struct {
	items []graph.UserMasteryInfo
	err   error
}

func (m *mockUserMasteryReader) GetUserMastery(_ context.Context, _ string) ([]graph.UserMasteryInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

func TestStatsService_BuildUserRadarStats_GraphUnavailable(t *testing.T) {
	svc := NewStatsService(nil)

	_, err := svc.BuildUserRadarStats("u1", 6)
	if !errors.Is(err, ErrStatsGraphUnavailable) {
		t.Fatalf("expected ErrStatsGraphUnavailable, got %v", err)
	}
}

func TestStatsService_BuildUserRadarStats_Error(t *testing.T) {
	svc := NewStatsService(&mockUserMasteryReader{err: errors.New("boom")})

	_, err := svc.BuildUserRadarStats("u1", 6)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestStatsService_BuildUserRadarStats_SortLimitAndScale(t *testing.T) {
	svc := NewStatsService(&mockUserMasteryReader{
		items: []graph.UserMasteryInfo{
			{SkillKey: "dp", Mastery: 0.5},
			{SkillKey: "graph", Mastery: 0.9},
			{SkillKey: "greedy", Mastery: 0.2},
		},
	})

	stats, err := svc.BuildUserRadarStats("u1", 2)
	if err != nil {
		t.Fatalf("BuildUserRadarStats failed: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 items, got %d", len(stats))
	}
	if stats[0].Subject != "graph" || stats[0].Value != 90 {
		t.Fatalf("unexpected first item: %+v", stats[0])
	}
	if stats[1].Subject != "dp" || stats[1].Value != 50 {
		t.Fatalf("unexpected second item: %+v", stats[1])
	}
	if stats[0].FullMark != 100 || stats[1].FullMark != 100 {
		t.Fatalf("unexpected full mark: %+v", stats)
	}
}

func TestStatsService_BuildUserRadarStats_DefaultLimit(t *testing.T) {
	svc := NewStatsService(&mockUserMasteryReader{
		items: []graph.UserMasteryInfo{
			{SkillKey: "s1", Mastery: 0.1},
			{SkillKey: "s2", Mastery: 0.2},
			{SkillKey: "s3", Mastery: 0.3},
			{SkillKey: "s4", Mastery: 0.4},
			{SkillKey: "s5", Mastery: 0.5},
			{SkillKey: "s6", Mastery: 0.6},
			{SkillKey: "s7", Mastery: 0.7},
		},
	})

	stats, err := svc.BuildUserRadarStats("u1", 0)
	if err != nil {
		t.Fatalf("BuildUserRadarStats failed: %v", err)
	}
	if len(stats) != 6 {
		t.Fatalf("expected default limit 6, got %d", len(stats))
	}
}
