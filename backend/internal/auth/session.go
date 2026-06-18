package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var sessionSecret []byte

const (
	sessionCookieName = "session"
	sessionDuration   = 24 * time.Hour
)

// InitSession loads the HMAC signing key from SESSION_SECRET.
func InitSession() error {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return errors.New("missing SESSION_SECRET environment variable")
	}
	sessionSecret = []byte(secret)
	return nil
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CreateSessionCookie builds an HMAC-signed cookie containing the email and expiry.
// No server-side store is needed — the signature prevents client-side tampering.
func CreateSessionCookie(email string) *http.Cookie {
	expiry := time.Now().Add(sessionDuration).Unix()
	payload := fmt.Sprintf("%s|%d", email, expiry)
	value := fmt.Sprintf("%s|%s", payload, sign(payload))

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(value)),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionDuration),
	}
}

// ReadSessionCookie validates the signature and expiry, returning the email on success.
func ReadSessionCookie(r *http.Request) (string, bool) {
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
	if !hmac.Equal([]byte(signature), []byte(sign(payload))) {
		return "", false
	}
	var expiry int64
	if _, err := fmt.Sscanf(expiryStr, "%d", &expiry); err != nil || time.Now().Unix() > expiry {
		return "", false
	}
	if !IsAllowed(email) {
		return "", false
	}
	return email, true
}

// ClearSessionCookie returns an expired cookie that deletes the session client-side.
func ClearSessionCookie() *http.Cookie {
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
