package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// sessionSecret signs session cookies to prevent tampering.
// It should be set via an environment variable in production.
var sessionSecret []byte

const sessionCookieName = "session"
const sessionDuration = 1 * 24 * time.Hour // 1 day

func initSession() error {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return fmt.Errorf("missing environment variable SESSION_SECRET")
	}
	sessionSecret = []byte(secret)
	return nil
}

// sign produces an HMAC-SHA256 signature of the payload, encoded using
// base64 URL-safe encoding.
func sign(payload string) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// createSessionCookie builds a cookie with the format "email|expiry|signature".
// This is intentionally simple (no server-side store or DB): the cookie
// contains the data and its signature ensures it was not tampered with
// client-side.
func createSessionCookie(email string) *http.Cookie {
	expiry := time.Now().Add(sessionDuration).Unix()
	payload := fmt.Sprintf("%s|%d", email, expiry)
	signature := sign(payload)
	value := fmt.Sprintf("%s|%s", payload, signature)

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(value)),
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // requires HTTPS (true on Render)
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionDuration),
	}
}

// readSessionCookie validates the signature and expiry, and returns the
// email if the cookie is valid and belongs to an allowed user.
func readSessionCookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}

	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", false
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", false
	}
	email, expiryStr, signature := parts[0], parts[1], parts[2]

	payload := fmt.Sprintf("%s|%s", email, expiryStr)
	expectedSignature := sign(payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return "", false // invalid signature, cookie may have been tampered
	}

	var expiry int64
	if _, err := fmt.Sscanf(expiryStr, "%d", &expiry); err != nil {
		return "", false
	}
	if time.Now().Unix() > expiry {
		return "", false // cookie expired
	}

	if !isAllowed(email) {
		return "", false // email is no longer (or never was) allowed
	}

	return email, true
}

// clearSessionCookie returns an expired cookie to log out the user.
func clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}
