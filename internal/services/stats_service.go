package services

import (
	"context"
	"errors"
	"sort"

	"patreonoj/internal/graph"
)

var ErrStatsGraphUnavailable = errors.New("graph service is not initialized")

type RadarStatItem struct {
	Subject  string  `json:"subject"`
	Value    float64 `json:"A"`
	FullMark float64 `json:"fullMark"`
}

type userMasteryReader interface {
	GetUserMastery(ctx context.Context, userID string) ([]graph.UserMasteryInfo, error)
}

type StatsService struct {
	graphService userMasteryReader
}

func NewStatsService(graphService userMasteryReader) *StatsService {
	return &StatsService{graphService: graphService}
}

func (s *StatsService) BuildUserRadarStats(userUUID string, limit int) ([]RadarStatItem, error) {
	if s.graphService == nil {
		return nil, ErrStatsGraphUnavailable
	}

	if limit <= 0 {
		limit = 6
	}

	masteries, err := s.graphService.GetUserMastery(context.Background(), userUUID)
	if err != nil {
		return nil, err
	}

	sort.Slice(masteries, func(i, j int) bool {
		return masteries[i].Mastery > masteries[j].Mastery
	})

	if len(masteries) > limit {
		masteries = masteries[:limit]
	}

	stats := make([]RadarStatItem, 0, len(masteries))
	for _, m := range masteries {
		stats = append(stats, RadarStatItem{
			Subject:  m.SkillKey,
			Value:    m.Mastery * 100,
			FullMark: 100,
		})
	}
	return stats, nil
}
