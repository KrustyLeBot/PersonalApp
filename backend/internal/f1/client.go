package f1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	baseURL   = "https://api.jolpi.ca/ergast/f1"
	userAgent = "PersonalApp-F1Tracker/1.0 (personal project)"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) get(url string, dst any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jolpica API returned %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// FetchSeason returns all races in the schedule for the given year.
func (c *Client) FetchSeason(year int) ([]Race, error) {
	var resp apiSeasonResponse
	if err := c.get(fmt.Sprintf("%s/%d.json", baseURL, year), &resp); err != nil {
		return nil, err
	}

	races := make([]Race, 0, len(resp.MRData.RaceTable.Races))
	for _, r := range resp.MRData.RaceTable.Races {
		round, _ := strconv.Atoi(r.Round)
		races = append(races, Race{
			Season:      year,
			Round:       round,
			RaceName:    r.RaceName,
			CircuitID:   r.Circuit.CircuitID,
			CircuitName: r.Circuit.CircuitName,
			Locality:    r.Circuit.Location.Locality,
			Country:     r.Circuit.Location.Country,
			RaceDate:    r.Date,
			RaceTime:    r.Time,
			QualiDate:   r.Qualifying.Date,
			QualiTime:   r.Qualifying.Time,
			SprintDate:  r.Sprint.Date,
			SprintTime:  r.Sprint.Time,
			HasSprint:   r.Sprint.Date != "",
		})
	}
	return races, nil
}

// FetchRaceResults returns the finishing order for a specific round.
func (c *Client) FetchRaceResults(year, round int) ([]RaceResult, error) {
	var resp apiResultsResponse
	if err := c.get(fmt.Sprintf("%s/%d/%d/results.json", baseURL, year, round), &resp); err != nil {
		return nil, err
	}

	if len(resp.MRData.RaceTable.Races) == 0 {
		return nil, nil
	}

	apiResults := resp.MRData.RaceTable.Races[0].Results
	results := make([]RaceResult, 0, len(apiResults))
	for _, r := range apiResults {
		pos, _ := strconv.Atoi(r.Position)
		pts, _ := strconv.ParseFloat(r.Points, 64)
		grid, _ := strconv.Atoi(r.Grid)
		laps, _ := strconv.Atoi(r.Laps)

		res := RaceResult{
			Season:           year,
			Round:            round,
			Position:         pos,
			DriverID:         r.Driver.DriverID,
			DriverCode:       r.Driver.Code,
			DriverGivenName:  r.Driver.GivenName,
			DriverFamilyName: r.Driver.FamilyName,
			ConstructorID:    r.Constructor.ConstructorID,
			ConstructorName:  r.Constructor.Name,
			Grid:             grid,
			Laps:             laps,
			Points:           pts,
			Status:           r.Status,
		}
		if r.FastestLap != nil {
			res.FastestLapRank, _ = strconv.Atoi(r.FastestLap.Rank)
			res.FastestLapTime = r.FastestLap.Time.Time
		}
		results = append(results, res)
	}
	return results, nil
}

// FetchQualifyingResults returns the qualifying order for a specific round.
func (c *Client) FetchQualifyingResults(year, round int) ([]QualifyingResult, error) {
	var resp apiQualifyingResponse
	if err := c.get(fmt.Sprintf("%s/%d/%d/qualifying.json", baseURL, year, round), &resp); err != nil {
		return nil, err
	}

	if len(resp.MRData.RaceTable.Races) == 0 {
		return nil, nil
	}

	raw := resp.MRData.RaceTable.Races[0].QualifyingResults
	results := make([]QualifyingResult, 0, len(raw))
	for _, r := range raw {
		pos, _ := strconv.Atoi(r.Position)
		results = append(results, QualifyingResult{
			Season:           year,
			Round:            round,
			Position:         pos,
			DriverID:         r.Driver.DriverID,
			DriverCode:       r.Driver.Code,
			DriverGivenName:  r.Driver.GivenName,
			DriverFamilyName: r.Driver.FamilyName,
			ConstructorID:    r.Constructor.ConstructorID,
			ConstructorName:  r.Constructor.Name,
			Q1:               r.Q1,
			Q2:               r.Q2,
			Q3:               r.Q3,
		})
	}
	return results, nil
}

// FetchDriverStandings returns the current driver championship standings.
func (c *Client) FetchDriverStandings() ([]DriverStanding, error) {
	var resp apiDriverStandingsResponse
	if err := c.get(fmt.Sprintf("%s/current/driverStandings.json", baseURL), &resp); err != nil {
		return nil, err
	}

	lists := resp.MRData.StandingsTable.StandingsLists
	if len(lists) == 0 {
		return nil, nil
	}

	raw := lists[0].DriverStandings
	standings := make([]DriverStanding, 0, len(raw))
	for _, s := range raw {
		pos, _ := strconv.Atoi(s.Position)
		pts, _ := strconv.ParseFloat(s.Points, 64)
		wins, _ := strconv.Atoi(s.Wins)
		constructor := ""
		if len(s.Constructors) > 0 {
			constructor = s.Constructors[0].Name
		}
		standings = append(standings, DriverStanding{
			Position:         pos,
			DriverID:         s.Driver.DriverID,
			DriverCode:       s.Driver.Code,
			DriverGivenName:  s.Driver.GivenName,
			DriverFamilyName: s.Driver.FamilyName,
			ConstructorName:  constructor,
			Points:           pts,
			Wins:             wins,
		})
	}
	return standings, nil
}

// FetchConstructorStandings returns the current constructor championship standings.
func (c *Client) FetchConstructorStandings() ([]ConstructorStanding, error) {
	var resp apiConstructorStandingsResponse
	if err := c.get(fmt.Sprintf("%s/current/constructorStandings.json", baseURL), &resp); err != nil {
		return nil, err
	}

	lists := resp.MRData.StandingsTable.StandingsLists
	if len(lists) == 0 {
		return nil, nil
	}

	raw := lists[0].ConstructorStandings
	standings := make([]ConstructorStanding, 0, len(raw))
	for _, s := range raw {
		pos, _ := strconv.Atoi(s.Position)
		pts, _ := strconv.ParseFloat(s.Points, 64)
		wins, _ := strconv.Atoi(s.Wins)
		standings = append(standings, ConstructorStanding{
			Position:        pos,
			ConstructorID:   s.Constructor.ConstructorID,
			ConstructorName: s.Constructor.Name,
			Points:          pts,
			Wins:            wins,
		})
	}
	return standings, nil
}
