package f1

import "time"

type Race struct {
	Season      int        `json:"season"`
	Round       int        `json:"round"`
	RaceName    string     `json:"raceName"`
	CircuitID   string     `json:"circuitId"`
	CircuitName string     `json:"circuitName"`
	Locality    string     `json:"locality"`
	Country     string     `json:"country"`
	RaceDate    string     `json:"raceDate"` // YYYY-MM-DD
	RaceTime    string     `json:"raceTime"` // HH:MM:SS or ""
	IsPast      bool       `json:"isPast"`
	FetchedAt   time.Time  `json:"fetchedAt"`
}

type RaceResult struct {
	Season            int     `json:"season"`
	Round             int     `json:"round"`
	Position          int     `json:"position"`
	DriverID          string  `json:"driverId"`
	DriverCode        string  `json:"driverCode"`
	DriverGivenName   string  `json:"driverGivenName"`
	DriverFamilyName  string  `json:"driverFamilyName"`
	ConstructorID     string  `json:"constructorId"`
	ConstructorName   string  `json:"constructorName"`
	Grid              int     `json:"grid"`
	Laps              int     `json:"laps"`
	Points            float64 `json:"points"`
	Status            string  `json:"status"`
	FastestLapRank    int     `json:"fastestLapRank"`
	FastestLapTime    string  `json:"fastestLapTime"`
}

type DriverStanding struct {
	Position         int     `json:"position"`
	DriverID         string  `json:"driverId"`
	DriverCode       string  `json:"driverCode"`
	DriverGivenName  string  `json:"driverGivenName"`
	DriverFamilyName string  `json:"driverFamilyName"`
	ConstructorName  string  `json:"constructorName"`
	Points           float64 `json:"points"`
	Wins             int     `json:"wins"`
}

type ConstructorStanding struct {
	Position        int     `json:"position"`
	ConstructorID   string  `json:"constructorId"`
	ConstructorName string  `json:"constructorName"`
	Points          float64 `json:"points"`
	Wins            int     `json:"wins"`
}

type QualifyingResult struct {
	Season           int    `json:"season"`
	Round            int    `json:"round"`
	Position         int    `json:"position"`
	DriverID         string `json:"driverId"`
	DriverCode       string `json:"driverCode"`
	DriverGivenName  string `json:"driverGivenName"`
	DriverFamilyName string `json:"driverFamilyName"`
	ConstructorID    string `json:"constructorId"`
	ConstructorName  string `json:"constructorName"`
	Q1               string `json:"q1"`
	Q2               string `json:"q2"`
	Q3               string `json:"q3"`
}

// --- Jolpica API response types ---

type apiQualifyingResponse struct {
	MRData struct {
		RaceTable struct {
			Season string `json:"season"`
			Round  string `json:"round"`
			Races  []struct {
				QualifyingResults []apiQualifyingResult `json:"QualifyingResults"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type apiQualifyingResult struct {
	Position string `json:"position"`
	Q1       string `json:"Q1"`
	Q2       string `json:"Q2"`
	Q3       string `json:"Q3"`
	Driver   struct {
		DriverID   string `json:"driverId"`
		Code       string `json:"code"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"Driver"`
	Constructor struct {
		ConstructorID string `json:"constructorId"`
		Name          string `json:"name"`
	} `json:"Constructor"`
}

type apiSeasonResponse struct {
	MRData struct {
		RaceTable struct {
			Season string    `json:"season"`
			Races  []apiRace `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type apiRace struct {
	Season   string `json:"season"`
	Round    string `json:"round"`
	RaceName string `json:"raceName"`
	Circuit  struct {
		CircuitID   string `json:"circuitId"`
		CircuitName string `json:"circuitName"`
		Location    struct {
			Locality string `json:"locality"`
			Country  string `json:"country"`
		} `json:"Location"`
	} `json:"Circuit"`
	Date    string       `json:"date"`
	Time    string       `json:"time"`
	Results []apiResult  `json:"Results"`
}

type apiResultsResponse struct {
	MRData struct {
		RaceTable struct {
			Season string    `json:"season"`
			Round  string    `json:"round"`
			Races  []apiRace `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type apiResult struct {
	Position     string `json:"position"`
	Points       string `json:"points"`
	Grid         string `json:"grid"`
	Laps         string `json:"laps"`
	Status       string `json:"status"`
	Driver       struct {
		DriverID   string `json:"driverId"`
		Code       string `json:"code"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"Driver"`
	Constructor struct {
		ConstructorID string `json:"constructorId"`
		Name          string `json:"name"`
	} `json:"Constructor"`
	FastestLap *struct {
		Rank string `json:"rank"`
		Time struct {
			Time string `json:"time"`
		} `json:"Time"`
	} `json:"FastestLap"`
}

type apiDriverStandingsResponse struct {
	MRData struct {
		StandingsTable struct {
			Season        string `json:"season"`
			StandingsLists []struct {
				DriverStandings []struct {
					Position    string `json:"position"`
					Points      string `json:"points"`
					Wins        string `json:"wins"`
					Driver      struct {
						DriverID   string `json:"driverId"`
						Code       string `json:"code"`
						GivenName  string `json:"givenName"`
						FamilyName string `json:"familyName"`
					} `json:"Driver"`
					Constructors []struct {
						Name string `json:"name"`
					} `json:"Constructors"`
				} `json:"DriverStandings"`
			} `json:"StandingsLists"`
		} `json:"StandingsTable"`
	} `json:"MRData"`
}

type apiConstructorStandingsResponse struct {
	MRData struct {
		StandingsTable struct {
			Season        string `json:"season"`
			StandingsLists []struct {
				ConstructorStandings []struct {
					Position    string `json:"position"`
					Points      string `json:"points"`
					Wins        string `json:"wins"`
					Constructor struct {
						ConstructorID string `json:"constructorId"`
						Name          string `json:"name"`
					} `json:"Constructor"`
				} `json:"ConstructorStandings"`
			} `json:"StandingsLists"`
		} `json:"StandingsTable"`
	} `json:"MRData"`
}
