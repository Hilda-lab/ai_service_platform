package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	jwtpkg "ai-service-platform/backend/pkg/jwt"
)

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Printf("[JWT] Missing authorization header from %s", c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{"message": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			log.Printf("[JWT] Invalid header format from %s: %s", c.ClientIP(), authHeader[:20])
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid authorization header"})
			c.Abort()
			return
		}

		claims, err := jwtpkg.ParseToken(parts[1], secret)
		if err != nil {
			log.Printf("[JWT] Token validation failed from %s: %v", c.ClientIP(), err)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token", "error": err.Error()})
			c.Abort()
			return
		}

		log.Printf("[JWT] ✓ Token validated for user %d from %s", claims.UserID, c.ClientIP())
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
