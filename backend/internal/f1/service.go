package f1

import (
	"log"
	"time"
)

type Service struct {
	repo   *Repo
	client *Client
}

func NewService(repo *Repo, client *Client) *Service {
	return &Service{repo: repo, client: client}
}

func currentSeason() int {
	return time.Now().UTC().Year()
}


// Refresh fetches season schedule, standings and any missing race results.
func (s *Service) Refresh() error {
	year := currentSeason()

	races, err := s.client.FetchSeason(year)
	if err != nil {
		return err
	}
	if err := s.repo.UpsertRaces(races); err != nil {
		return err
	}

	// Fetch race results for past rounds not yet stored.
	rounds, err := s.repo.RoundsWithoutResults(year)
	if err != nil {
		return err
	}
	for _, round := range rounds {
		results, err := s.client.FetchRaceResults(year, round)
		if err != nil {
			log.Printf("f1: fetch results season=%d round=%d: %v", year, round, err)
			continue
		}
		if len(results) == 0 {
			continue
		}
		if err := s.repo.UpsertRaceResults(results); err != nil {
			return err
		}
	}

	// Fetch qualifying results for past rounds not yet stored.
	qualRounds, err := s.repo.RoundsWithoutQualifying(year)
	if err != nil {
		return err
	}
	for _, round := range qualRounds {
		results, err := s.client.FetchQualifyingResults(year, round)
		if err != nil {
			log.Printf("f1: fetch qualifying season=%d round=%d: %v", year, round, err)
			continue
		}
		if len(results) == 0 {
			continue
		}
		if err := s.repo.UpsertQualifyingResults(results); err != nil {
			return err
		}
	}

	driverStandings, err := s.client.FetchDriverStandings()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertDriverStandings(year, driverStandings); err != nil {
		return err
	}

	constructorStandings, err := s.client.FetchConstructorStandings()
	if err != nil {
		return err
	}
	if err := s.repo.UpsertConstructorStandings(year, constructorStandings); err != nil {
		return err
	}

	return s.repo.RecordDailyRefresh()
}
