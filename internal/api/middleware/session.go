package middleware

import (
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/crypto"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	// ContextKeySessionAppID is the key used to store the application ID
	// in gin.Context after session validation.
	ContextKeySessionAppID = "session_application_id"
	ContextKeySessionID    = "session_id"

	// SessionTokenHeader is the HTTP header name customers use to send their token.
	SessionTokenHeader = "X-Session-Token"
)

// RequireCustomerSession validates the customer's session token.
// Used to protect Steps 3–7 of the application form.
//
// HOW IT WORKS:
//  1. Reads the raw token from X-Session-Token header
//  2. Hashes it with SHA-256 (same way it was stored in DB)
//  3. Looks up the hash in customer_sessions table
//  4. Checks the session hasn't expired or been revoked
//  5. Verifies the session belongs to the application_id in the URL
//
// This prevents a customer from using someone else's session token
// to access a different application.
func RequireCustomerSession(appRepo repository.ApplicationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract raw token from header
		rawToken := c.GetHeader(SessionTokenHeader)
		if rawToken == "" {
			response.Unauthorized(c, "Session token is required. Please start from Step 1.")
			c.Abort()
			return
		}

		// Hash the raw token for DB lookup
		tokenHash := crypto.HashToken(rawToken)

		// Look up session in database
		session, err := appRepo.FindSessionByTokenHash(c.Request.Context(), tokenHash)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired session. Please start a new application.")
			c.Abort()
			return
		}

		// Check session hasn't expired
		if time.Now().After(session.ExpiresAt) {
			response.Unauthorized(c, "Your session has expired. Please start a new application.")
			c.Abort()
			return
		}

		// Verify token belongs to the application in the URL
		// This prevents token reuse across different applications
		urlAppID := c.Param("id")
		if urlAppID != "" && session.ApplicationID != urlAppID {
			response.Forbidden(c, "This session token does not belong to this application.")
			c.Abort()
			return
		}

		// Store session info in context for downstream handlers
		c.Set(ContextKeySessionAppID, session.ApplicationID)
		c.Set(ContextKeySessionID, session.ID)

		c.Next()
	}
}

// GetSessionAppID extracts the application ID from the session context.
func GetSessionAppID(c *gin.Context) string {
	val, _ := c.Get(ContextKeySessionAppID)
	id, _ := val.(string)
	return id
}
