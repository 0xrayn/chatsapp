package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"chatapp/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// jwtSecret is loaded once at startup via MustLoadJWTSecret().
var jwtSecret []byte

// blacklistRepo is injected at startup via SetBlacklistRepo().
// AuthMiddleware uses it to reject revoked tokens.
var blacklistRepo domain.TokenBlacklistRepository

// MustLoadJWTSecret caches the JWT signing secret.
// Panics if absent or shorter than 32 characters — server must never start without it.
//
//	openssl rand -hex 32
func MustLoadJWTSecret(secret string) {
	if len(secret) < 32 {
		panic("JWT_SECRET env variable is required and must be at least 32 characters.\n" +
			"Generate one with: openssl rand -hex 32")
	}
	jwtSecret = []byte(secret)
}

// SetBlacklistRepo injects the token blacklist repository used by AuthMiddleware.
func SetBlacklistRepo(repo domain.TokenBlacklistRepository) {
	blacklistRepo = repo
}

type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
	// JTI (JWT ID) is a unique per-token identifier used for revocation.
	// Stored in the token_blacklist table on logout.
}

func GenerateToken(userID uuid.UUID, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // unique JTI per token
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format. Use: Bearer <token>"})
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Check blacklist — reject tokens that were explicitly revoked on logout.
		if blacklistRepo != nil && claims.ID != "" {
			revoked, err := blacklistRepo.IsRevoked(claims.ID)
			if err == nil && revoked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked. Please log in again."})
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("token_jti", claims.ID)
		c.Set("token_exp", claims.ExpiresAt.Time)
		c.Next()
	}
}

func GetUserID(c *gin.Context) uuid.UUID {
	return c.MustGet("user_id").(uuid.UUID)
}

func GetUsername(c *gin.Context) string {
	return c.MustGet("username").(string)
}

// WSAuthMiddleware authenticates WebSocket upgrade requests.
// Token is sent as the first WebSocket message after connect — never in the URL.
func WSAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fallback: Authorization header (non-browser / API clients)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				claims, err := ParseToken(parts[1])
				if err == nil {
					// Blacklist check for header-based WS auth too
					if blacklistRepo != nil && claims.ID != "" {
						revoked, err := blacklistRepo.IsRevoked(claims.ID)
						if err == nil && revoked {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
							return
						}
					}
					c.Set("user_id", claims.UserID)
					c.Set("username", claims.Username)
					c.Set("token_jti", claims.ID)
					c.Next()
					return
				}
			}
		}

		// No header — WS handler will request first-message auth
		c.Set("ws_pending_auth", true)
		c.Next()
	}
}
