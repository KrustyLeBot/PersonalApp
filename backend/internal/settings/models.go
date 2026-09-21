package settings

var KnownFeatures = []string{"portfolio", "telework", "lol-calendar", "f1", "walk"}

type UserFeatures struct {
	Enabled []string `json:"enabled"`
}
