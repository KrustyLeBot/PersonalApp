package lolcalendar

import "time"

type League struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	LeagueID string `json:"leagueId"`
	Region   string `json:"region"`
	ImageURL string `json:"imageUrl"`
	Enabled  bool   `json:"enabled"`
}

type Team struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	ImageURL string `json:"imageUrl"`
	GameWins int    `json:"gameWins"`
	Outcome  string `json:"outcome"` // "win", "loss", or ""
}

type Match struct {
	MatchID     string    `json:"matchId"`
	LeagueName  string    `json:"leagueName"`
	LeagueSlug  string    `json:"leagueSlug"`
	Team1       Team      `json:"team1"`
	Team2       Team      `json:"team2"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Stage       string    `json:"stage"`
	BestOf      int       `json:"bestOf"`
	State            string    `json:"state"` // "unstarted", "inProgress", "completed"
	IsSpoiler        bool      `json:"isSpoiler"`
	SpoilerDismissed bool      `json:"spoilerDismissed"`
	FetchedAt        time.Time `json:"fetchedAt"`
}


type GameVOD struct {
	GameNumber int    `json:"gameNumber"`
	Provider   string `json:"provider"` // "youtube" or "twitch"
	Parameter  string `json:"parameter"`
	StartSecs  int    `json:"startSecs"`
}

// API response types — used only for JSON parsing.

type apiEventDetailsResponse struct {
	Data struct {
		Event struct {
			Match struct {
				Games []struct {
					Number int      `json:"number"`
					State  string   `json:"state"`
					VODs   []apiVOD `json:"vods"`
				} `json:"games"`
			} `json:"match"`
		} `json:"event"`
	} `json:"data"`
}

type apiLeaguesResponse struct {
	Data struct {
		Leagues []struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			Name   string `json:"name"`
			Region string `json:"region"`
			Image  string `json:"image"`
		} `json:"leagues"`
	} `json:"data"`
}

type apiScheduleResponse struct {
	Data struct {
		Schedule struct {
			Pages  apiPages   `json:"pages"`
			Events []apiEvent `json:"events"`
		} `json:"schedule"`
	} `json:"data"`
}

type apiPages struct {
	Older string `json:"older"`
	Newer string `json:"newer"`
}

type apiEvent struct {
	StartTime string `json:"startTime"`
	State     string `json:"state"`
	Type      string `json:"type"`
	BlockName string `json:"blockName"`
	League    struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"league"`
	Match struct {
		ID    string   `json:"id"`
		Flags []string `json:"flags"`
		Teams []struct {
			Name   string `json:"name"`
			Code   string `json:"code"`
			Image  string `json:"image"`
			Result *struct {
				Outcome  *string `json:"outcome"`
				GameWins int     `json:"gameWins"`
			} `json:"result"`
		} `json:"teams"`
		Strategy struct {
			Count int `json:"count"`
		} `json:"strategy"`
	} `json:"match"`
}
