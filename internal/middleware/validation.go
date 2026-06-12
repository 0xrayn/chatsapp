package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

// BindJSONOrError binds JSON and writes a formatted 400 response on failure.
// Returns false if binding failed (handler should return immediately).
func BindJSONOrError(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return false
	}
	return true
}

// ValidateUsername checks the username only contains letters, numbers, underscores (3-30 chars)
func ValidateUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

// ValidatePasswordStrength checks the password has at least one letter and one digit
func ValidatePasswordStrength(password string) bool {
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// ValidationErrors collects field-level error messages
type ValidationErrors map[string]string

func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// RespondValidationError writes a 400 response with field errors
func RespondValidationError(c *gin.Context, errs ValidationErrors) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":  "Validation failed",
		"fields": errs,
	})
}

// SanitizeString trims whitespace and collapses internal spacing
func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}
