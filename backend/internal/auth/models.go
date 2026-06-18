package auth

// googleUserInfo is the subset of Google's userinfo v2 response we consume.
// Kept package-private — callers only interact with the email string.
type googleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}
