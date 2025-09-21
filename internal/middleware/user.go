package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	UserCookieName = "qng_agent_user_id"
	UserIDKey      = "user_id"
	CookieMaxAge   = 365 * 24 * 3600 // 1 year
)

// UserMiddleware extracts or generates user ID from cookie
func UserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userID string

		// Try to get user ID from cookie
		if cookie, err := c.Cookie(UserCookieName); err == nil && cookie != "" {
			userID = cookie
		}

		// If no valid cookie, generate new user ID
		if userID == "" {
			userID = uuid.New().String()

			// Set cookie with proper settings
			c.SetCookie(
				UserCookieName,     // name
				userID,             // value
				CookieMaxAge,       // maxAge
				"/",                // path
				"",                 // domain (empty for same-origin)
				false,              // secure (set to true in production with HTTPS)
				true,               // httpOnly
			)
		}

		// Store user ID in context for handlers
		c.Set(UserIDKey, userID)
		c.Next()
	}
}

// GetUserID extracts user ID from gin context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(UserIDKey); exists {
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}