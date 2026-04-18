package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"piliminusb/config"
	"piliminusb/response"
)

const ContextUserID = "user_id"

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string
		if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		} else if q := c.Query("token"); q != "" {
			// WebSocket handshakes can't set custom headers in browsers; accept
			// the JWT via ?token=... as a fallback for WS endpoints like sauc.
			tokenStr = q
		} else {
			response.Unauthorized(c, "missing or invalid Authorization header")
			c.Abort()
			return
		}

		secret := config.Get().JWT.Secret

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Unauthorized(c, "invalid token claims")
			c.Abort()
			return
		}

		userID, ok := claims["user_id"].(float64)
		if !ok {
			response.Unauthorized(c, "invalid user_id in token")
			c.Abort()
			return
		}

		c.Set(ContextUserID, uint(userID))
		c.Next()
	}
}

// GetUserID extracts the authenticated user's ID from the context.
func GetUserID(c *gin.Context) uint {
	id, _ := c.Get(ContextUserID)
	return id.(uint)
}
