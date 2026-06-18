package auth

import "net/http"

// HandlerFunc is an http.HandlerFunc that also receives the authenticated user's email.
// All protected route handlers must match this signature.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, email string)

// RequireAuth wraps a HandlerFunc and returns a standard http.HandlerFunc.
// Requests with a missing, invalid, or expired session are rejected with 401.
// This is the single enforcement point for authentication — every non-public
// route must be wrapped here.
func RequireAuth(next HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, ok := ReadSessionCookie(r)
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		next(w, r, email)
	}
}
