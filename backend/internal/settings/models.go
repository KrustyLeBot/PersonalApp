package settings

var KnownFeatures = []string{"portfolio", "telework", "lol-calendar", "f1"}

type UserFeatures struct {
	Enabled []string `json:"enabled"`
}
