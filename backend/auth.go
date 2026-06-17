package main

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

// allowedEmails is the whitelist of emails allowed to access the app.
// The list is loaded from the ALLOWED_EMAILS environment variable.
var allowedEmails = map[string]bool{}

// oauthConfig is built once at startup from environment variables.
var oauthConfig *oauth2.Config

func initAllowedEmails() error {
	raw := os.Getenv("ALLOWED_EMAILS")
	if raw == "" {
		return errors.New("missing ALLOWED_EMAILS environment variable")
	}

	for _, email := range strings.Split(raw, ",") {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" {
			continue
		}
		allowedEmails[normalized] = true
	}

	if len(allowedEmails) == 0 {
		return errors.New("ALLOWED_EMAILS contains no valid emails")
	}
	return nil
}

func initOAuth() error {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL") // ex: https://ton-app.onrender.com/auth/callback

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return errors.New("missing GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET or GOOGLE_REDIRECT_URL environment variables")
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

// googleUserInfo matches the response from Google's userinfo endpoint.
type googleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

// fetchGoogleUserInfo exchanges the OAuth code for a token, then queries the
// Google userinfo API to obtain the authenticated user's email.
//
// We use the userinfo endpoint rather than manually decoding the id_token JWT
// and fetching Google's JWKS certificates: the official library
// golang.org/x/oauth2 handles the code exchange and authenticated HTTP calls,
// avoiding reimplementing JWT signature validation (simpler and with less
// surface for security mistakes).
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
		return nil, errors.New("non-200 response from Google userinfo API")
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

// isAllowed checks that the email is in the whitelist, comparing
// in lowercase and trimming surrounding spaces.
func isAllowed(email string) bool {
	return allowedEmails[strings.ToLower(strings.TrimSpace(email))]
}
