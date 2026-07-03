package authutil

import (
	"net/http"
	"strings"

	"github.com/sapliy/sapliy-core/pkg/jwtutil"
)

// ExtractUserID retrieves the UserID from the request.
// It prioritizes the X-User-ID header (injected by the Gateway)
// and falls back to validating the JWT in the Authorization header or auth-token cookie.
func ExtractUserID(r *http.Request) (string, error) {
	// 1. Check X-User-ID header (Gateway injection)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	// 2. Fallback: Check Authorization header
	authHeader := r.Header.Get("Authorization")
	tokenString := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		// 3. Fallback: Check Cookie
		cookie, err := r.Cookie("auth-token")
		if err == nil {
			tokenString = cookie.Value
		}
	}

	if tokenString == "" {
		return "", nil
	}

	claims, err := jwtutil.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
