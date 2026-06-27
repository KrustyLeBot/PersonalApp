package lolcalendar

import (
	"sync"
	"time"
)

const pastDays = 30
const liveWindow = 30 * time.Minute

type Service struct {
	repo   *Repo
	client *Client

	liveRefreshMu   sync.Mutex
	lastLiveRefresh time.Time
}

func NewService(repo *Repo, client *Client) *Service {
	return &Service{repo: repo, client: client}
}

func (s *Service) FetchAllLeagues() ([]League, error) {
	return s.client.FetchAllLeagues()
}

func (s *Service) FetchVODs(matchID string) ([]GameVOD, error) {
	return s.client.FetchVODs(matchID)
}

// RefreshLive fetches only a short window around now from Riot and upserts.
// Throttled to once per minute to avoid hammering the API.
func (s *Service) RefreshLive(email string) error {
	s.liveRefreshMu.Lock()
	if time.Since(s.lastLiveRefresh) < time.Minute {
		s.liveRefreshMu.Unlock()
		return nil
	}
	s.lastLiveRefresh = time.Now()
	s.liveRefreshMu.Unlock()

	leagueIDs, err := s.repo.GetEnabledLeagueIDs(email)
	if err != nil {
		return err
	}
	// Fetch only 2 days to stay fast — live window only needs recent data.
	matches, err := s.client.FetchSchedule(leagueIDs, 2)
	if err != nil {
		return err
	}
	return s.repo.Upsert(matches, email)
}

func (s *Service) Refresh(email string) error {
	leagueIDs, err := s.repo.GetEnabledLeagueIDs(email)
	if err != nil {
		return err
	}
	matches, err := s.client.FetchSchedule(leagueIDs, pastDays)
	if err != nil {
		return err
	}
	if err := s.repo.Upsert(matches, email); err != nil {
		return err
	}
	return s.repo.RecordDailyRefresh(email)
}
