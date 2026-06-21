package lolcalendar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	apiKey      = "0TvQnueqKa5mxJntVWt0w4LpLfEkrV1Ta8rQBb9Z"
	scheduleURL = "https://esports-api.lolesports.com/persisted/gw/getSchedule"
	leaguesURL  = "https://esports-api.lolesports.com/persisted/gw/getLeagues"
	detailsURL  = "https://esports-api.lolesports.com/persisted/gw/getEventDetails"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}}
}

// FetchSchedule retrieves matches for the given league IDs covering the past
// pastDays days. Pages backwards via "older" tokens until the cutoff is reached.
func (c *Client) FetchSchedule(leagueIDs []string, pastDays int) ([]Match, error) {
	if len(leagueIDs) == 0 {
		return nil, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -pastDays)
	ids := strings.Join(leagueIDs, ",")
	var all []Match

	pageToken := ""
	for {
		events, pages, err := c.fetchPage(ids, pageToken)
		if err != nil {
			return nil, err
		}

		reachedCutoff := false
		for _, e := range events {
			if e.Type != "match" {
				continue
			}
			t, err := time.Parse(time.RFC3339, e.StartTime)
			if err != nil {
				continue
			}
			if t.Before(cutoff) {
				reachedCutoff = true
				continue
			}
			all = append(all, toMatch(e, t))
		}

		if reachedCutoff || pages.Older == "" {
			break
		}
		pageToken = pages.Older
	}

	return all, nil
}

func (c *Client) fetchPage(leagueIDs, pageToken string) ([]apiEvent, apiPages, error) {
	url := fmt.Sprintf("%s?hl=en-US&leagueId=%s", scheduleURL, leagueIDs)
	if pageToken != "" {
		url += "&pageToken=" + pageToken
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, apiPages{}, err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, apiPages{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiPages{}, fmt.Errorf("lolesports API returned %d", resp.StatusCode)
	}

	var result apiScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, apiPages{}, err
	}

	s := result.Data.Schedule
	return s.Events, s.Pages, nil
}

// FetchAllLeagues returns the full list of leagues from the Riot API.
func (c *Client) FetchAllLeagues() ([]League, error) {
	req, err := http.NewRequest("GET", leaguesURL+"?hl=en-US", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lolesports API returned %d", resp.StatusCode)
	}

	var result apiLeaguesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	leagues := make([]League, 0, len(result.Data.Leagues))
	for _, l := range result.Data.Leagues {
		leagues = append(leagues, League{
			Slug:     l.Slug,
			Name:     l.Name,
			LeagueID: l.ID,
			Region:   l.Region,
			ImageURL: l.Image,
		})
	}
	return leagues, nil
}

// FetchVODs returns one GameVOD per completed game for a match.
// Locale preference: fr-FR → en-US → first available.
func (c *Client) FetchVODs(matchID string) ([]GameVOD, error) {
	url := fmt.Sprintf("%s?hl=fr-FR&id=%s", detailsURL, matchID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lolesports API returned %d", resp.StatusCode)
	}

	var result apiEventDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var vods []GameVOD
	for _, g := range result.Data.Event.Match.Games {
		if g.State != "completed" {
			continue
		}
		vod := pickVOD(g.VODs)
		if vod == nil {
			continue
		}
		startSecs := 0
		if vod.StartMs != nil {
			startSecs = int(*vod.StartMs / 1000)
		}
		vods = append(vods, GameVOD{
			GameNumber: g.Number,
			Provider:   vod.Provider,
			Parameter:  vod.Parameter,
			StartSecs:  startSecs,
		})
	}
	return vods, nil
}

type apiVOD struct {
	Locale    string `json:"locale"`
	Provider  string `json:"provider"`
	Parameter string `json:"parameter"`
	StartMs   *int64 `json:"startMillis"`
}

// pickVOD selects the best VOD with priority: YouTube fr-FR > YouTube en-US >
// any YouTube > Twitch fr-FR > Twitch en-US > any Twitch > first available.
func pickVOD(vods []apiVOD) *apiVOD {
	ranks := map[string]int{}
	for i, v := range vods {
		yt := v.Provider == "youtube"
		fr := v.Locale == "fr-FR"
		en := v.Locale == "en-US"
		switch {
		case yt && fr:
			ranks[fmt.Sprintf("%d", i)] = 0
		case yt && en:
			ranks[fmt.Sprintf("%d", i)] = 1
		case yt:
			ranks[fmt.Sprintf("%d", i)] = 2
		case fr:
			ranks[fmt.Sprintf("%d", i)] = 3
		case en:
			ranks[fmt.Sprintf("%d", i)] = 4
		default:
			ranks[fmt.Sprintf("%d", i)] = 5
		}
	}
	best := -1
	bestRank := 99
	for i := range vods {
		r := ranks[fmt.Sprintf("%d", i)]
		if r < bestRank {
			bestRank = r
			best = i
		}
	}
	if best >= 0 {
		return &vods[best]
	}
	return nil
}

func toMatch(e apiEvent, t time.Time) Match {
	isSpoiler := false
	for _, f := range e.Match.Flags {
		if f == "isSpoiler" {
			isSpoiler = true
			break
		}
	}

	var team1, team2 Team
	if len(e.Match.Teams) > 0 {
		team1 = toTeam(e.Match.Teams[0])
	}
	if len(e.Match.Teams) > 1 {
		team2 = toTeam(e.Match.Teams[1])
	}

	return Match{
		MatchID:     e.Match.ID,
		LeagueName:  e.League.Name,
		LeagueSlug:  e.League.Slug,
		Team1:       team1,
		Team2:       team2,
		ScheduledAt: t,
		Stage:       e.BlockName,
		BestOf:      e.Match.Strategy.Count,
		State:       e.State,
		IsSpoiler:   isSpoiler,
		FetchedAt:   time.Now().UTC(),
	}
}

func toTeam(t struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Image  string `json:"image"`
	Result *struct {
		Outcome  *string `json:"outcome"`
		GameWins int     `json:"gameWins"`
	} `json:"result"`
}) Team {
	team := Team{
		Name:     t.Name,
		Code:     t.Code,
		ImageURL: t.Image,
	}
	if t.Result != nil {
		team.GameWins = t.Result.GameWins
		if t.Result.Outcome != nil {
			team.Outcome = *t.Result.Outcome
		}
	}
	return team
}
