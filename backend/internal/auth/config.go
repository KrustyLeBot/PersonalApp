package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	allowedEmails = map[string]bool{}
	oauthConfig   *oauth2.Config
)

// InitAllowedEmails parses ALLOWED_EMAILS (comma-separated) into the whitelist.
func InitAllowedEmails() error {
	raw := os.Getenv("ALLOWED_EMAILS")
	if raw == "" {
		return errors.New("missing ALLOWED_EMAILS environment variable")
	}
	for _, email := range strings.Split(raw, ",") {
		if n := strings.ToLower(strings.TrimSpace(email)); n != "" {
			allowedEmails[n] = true
		}
	}
	if len(allowedEmails) == 0 {
		return errors.New("ALLOWED_EMAILS contains no valid emails")
	}
	return nil
}

// InitOAuth builds the OAuth2 config from environment variables.
func InitOAuth() error {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return errors.New("missing GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET or GOOGLE_REDIRECT_URL")
	}
	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	return nil
}

// IsAllowed reports whether an email is in the whitelist.
func IsAllowed(email string) bool {
	return allowedEmails[strings.ToLower(strings.TrimSpace(email))]
}

// fetchGoogleUserInfo exchanges an OAuth code for a token, then retrieves the
// user's email from Google's userinfo endpoint. Using the official userinfo API
// avoids manual JWT/JWKS validation.
func fetchGoogleUserInfo(ctx context.Context, code string) (*googleUserInfo, error) {
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	client := oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("non-200 from Google userinfo")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
