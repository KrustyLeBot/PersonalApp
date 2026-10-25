package export

import (
	"archive/zip"
	"encoding/json"
	"io"
	"time"
)

// Service assembles the export archive.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// WriteZip streams the user's portfolio, telework and walk data as a zip of JSON files.
func (s *Service) WriteZip(w io.Writer, email string) error {
	portfolio, err := s.repo.GetPortfolio(email)
	if err != nil {
		return err
	}
	telework, err := s.repo.GetTelework(email)
	if err != nil {
		return err
	}
	walk, err := s.repo.GetWalk(email)
	if err != nil {
		return err
	}

	manifest := Manifest{
		FormatVersion: FormatVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Files:         []string{PortfolioFile, TeleworkFile, WalkFile},
	}

	zw := zip.NewWriter(w)
	for _, f := range []struct {
		name string
		data any
	}{
		{ManifestFile, manifest},
		{PortfolioFile, portfolio},
		{TeleworkFile, telework},
		{WalkFile, walk},
	} {
		if err := writeJSON(zw, f.name, f.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
