// Package middleware contains Gin middleware functions.
// Middleware runs before (or after) your route handlers and handles
// cross-cutting concerns like authentication, logging, and error recovery.
package middleware

import (
	"strings"

	"github.com/cappyHoding/ptdpn-eform-service/pkg/jwt"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// Context keys for values stored in gin.Context.
// Using typed constants prevents key collisions and typos.
const (
	ContextKeyUserID   = "auth_user_id"
	ContextKeyUsername = "auth_username"
	ContextKeyRole     = "auth_role"
	ContextKeyAppID    = "session_application_id"
)

// RequireInternalAuth validates the JWT Bearer token for internal staff endpoints.
//
// HOW IT WORKS:
//  1. Extracts the token from the Authorization header
//  2. Verifies the token signature using the RSA public key
//  3. Sets user claims in gin.Context for downstream handlers
//
// On failure: responds with 401 and aborts the request chain.
// On success: calls c.Next() to continue to the route handler.
func RequireInternalAuth(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract "Bearer <token>" from Authorization header
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				tokenString = parts[1]
			}
		}

		// Fallback ke query param ?token= (untuk <img src> tag yang tidak bisa kirim header)
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		// Kalau keduanya kosong, reject
		if tokenString == "" {
			response.Unauthorized(c, "Authorization header is required")
			c.Abort()
			return
		}

		// Verify the token — sama seperti sebelumnya
		claims, err := jwtManager.VerifyAccessToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)

		c.Next()

	}
}

// RequireRole checks that the authenticated user has one of the allowed roles.
// Must be used AFTER RequireInternalAuth middleware.
//
// Usage:
//
//	adminGroup.Use(middleware.RequireRole("admin"))
//	approvalGroup.Use(middleware.RequireRole("supervisor", "admin"))
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	// Build a set for O(1) lookup
	roleSet := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			// This should never happen if RequireInternalAuth ran first,
			// but we handle it defensively
			response.Unauthorized(c, "Authentication context missing")
			c.Abort()
			return
		}

		userRole, ok := role.(string)
		if !ok || !roleSet[userRole] {
			response.Forbidden(c, "Your role does not have access to this resource")
			c.Abort()
			return
		}

		c.Next()
	}
}

// Helper functions for handlers to extract auth context cleanly

// GetUserID extracts the authenticated user's ID from the gin context.
func GetUserID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyUserID)
	userID, _ := id.(string)
	return userID
}

// GetUsername extracts the authenticated user's username from the gin context.
func GetUsername(c *gin.Context) string {
	u, _ := c.Get(ContextKeyUsername)
	username, _ := u.(string)
	return username
}

// GetRole extracts the authenticated user's role from the gin context.
func GetRole(c *gin.Context) string {
	r, _ := c.Get(ContextKeyRole)
	role, _ := r.(string)
	return role
}
