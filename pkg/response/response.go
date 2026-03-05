// Package response provides standardized JSON response helpers for all API endpoints.
//
// EVERY response from this API follows this exact JSON structure:
//
//	{
//	    "success": true,
//	    "message": "Application created successfully",
//	    "data": { ... },       // present on success
//	    "error": null,         // present on error
//	    "meta": { ... }        // present for paginated lists
//	}
//
// WHY STANDARDIZE?
// The React frontend can write a single API interceptor that handles all
// responses uniformly — it always knows where to find the data, the message,
// and any errors. This is much easier to work with than each endpoint
// having its own response shape.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ─── Response Structs ────────────────────────────────────────────────────────

// Response is the standard API response envelope.
// All fields are exported so encoding/json can serialize them.
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`  // omitempty: not included if nil
	Error   *ErrorInfo  `json:"error,omitempty"` // omitempty: not included if nil
	Meta    *Meta       `json:"meta,omitempty"`  // omitempty: not included if nil
}

// ErrorInfo contains structured error details.
// Separating error info from the message allows the frontend to
// display field-specific validation errors in form inputs.
type ErrorInfo struct {
	Code    string            `json:"code"`              // Machine-readable code, e.g. "VALIDATION_ERROR"
	Details map[string]string `json:"details,omitempty"` // Field-specific errors, e.g. {"nik": "must be 16 digits"}
}

// Meta contains pagination information for list endpoints.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ─── Success Responses ───────────────────────────────────────────────────────

// OK sends a 200 response with data.
// Use for: GET requests that return a resource.
func OK(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created sends a 201 response.
// Use for: POST requests that create a new resource (application, user, etc.)
func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// OKWithMeta sends a 200 response with pagination metadata.
// Use for: List endpoints in the admin dashboard.
func OKWithMeta(c *gin.Context, message string, data interface{}, meta *Meta) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// NoContent sends a 204 response (no body).
// Use for: DELETE or update operations where no data needs to be returned.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ─── Error Responses ─────────────────────────────────────────────────────────

// BadRequest sends a 400 response.
// Use for: Invalid input, missing required fields, malformed JSON.
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "BAD_REQUEST",
		},
	})
}

// ValidationError sends a 400 response with field-specific validation errors.
// Use for: Form validation failures. The `details` map key is the field name.
//
// Example:
//
//	ValidationError(c, "Please check your input", map[string]string{
//	    "nik":   "NIK must be exactly 16 digits",
//	    "email": "Email format is invalid",
//	})
func ValidationError(c *gin.Context, message string, details map[string]string) {
	c.JSON(http.StatusUnprocessableEntity, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code:    "VALIDATION_ERROR",
			Details: details,
		},
	})
}

// Unauthorized sends a 401 response.
// Use for: Missing or invalid JWT token / session token.
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "Authentication required"
	}
	c.JSON(http.StatusUnauthorized, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "UNAUTHORIZED",
		},
	})
}

// Forbidden sends a 403 response.
// Use for: Authenticated user lacks permission (e.g. operator trying to approve).
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	c.JSON(http.StatusForbidden, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "FORBIDDEN",
		},
	})
}

// NotFound sends a 404 response.
// Use for: Resource doesn't exist (application ID not found, etc.)
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "Resource not found"
	}
	c.JSON(http.StatusNotFound, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "NOT_FOUND",
		},
	})
}

// Conflict sends a 409 response.
// Use for: Duplicate resource (NIK already has a pending application, etc.)
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "CONFLICT",
		},
	})
}

// UnprocessableEntity sends a 422 response.
// Use for: Business rule violations (e.g. trying to approve an already-approved application).
func UnprocessableEntity(c *gin.Context, message string) {
	c.JSON(http.StatusUnprocessableEntity, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "UNPROCESSABLE_ENTITY",
		},
	})
}

// TooManyRequests sends a 429 response.
// Use for: Rate limit exceeded.
func TooManyRequests(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, Response{
		Success: false,
		Message: "Too many requests. Please slow down and try again.",
		Error: &ErrorInfo{
			Code: "RATE_LIMIT_EXCEEDED",
		},
	})
}

// InternalError sends a 500 response.
// Use for: Unexpected errors (database down, VIDA API timeout, etc.)
//
// IMPORTANT: In production, never expose the raw error message to the client.
// The `err` parameter is only for internal logging — pass nil to omit the detail.
func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "An unexpected error occurred. Please try again later."
	}
	c.JSON(http.StatusInternalServerError, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "INTERNAL_ERROR",
		},
	})
}

// ServiceUnavailable sends a 503 response.
// Use for: VIDA service is down, maintenance mode, etc.
func ServiceUnavailable(c *gin.Context, message string) {
	if message == "" {
		message = "Service temporarily unavailable. Please try again later."
	}
	c.JSON(http.StatusServiceUnavailable, Response{
		Success: false,
		Message: message,
		Error: &ErrorInfo{
			Code: "SERVICE_UNAVAILABLE",
		},
	})
}

// ─── Helper ──────────────────────────────────────────────────────────────────

// NewMeta creates a pagination Meta struct.
// Call this from list service methods when returning paginated results.
//
//	meta := response.NewMeta(page, perPage, totalCount)
func NewMeta(page, perPage int, total int64) *Meta {
	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}
	return &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}
