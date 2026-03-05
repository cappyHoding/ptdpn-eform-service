package handler

import (
	"errors"
	"net/http"

	"github.com/cappyHoding/ptdpn-eform-service/internal/api/middleware"
	"github.com/cappyHoding/ptdpn-eform-service/internal/service"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles internal staff authentication endpoints.
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler creates a new AuthHandler with its required service.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type loginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// Login handles POST /api/v1/admin/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Username and password are required")
		return
	}

	result, err := h.authService.Login(c.Request.Context(), service.LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Unauthorized(c, "Invalid username or password")
		case errors.Is(err, service.ErrAccountInactive):
			response.Forbidden(c, "Your account has been deactivated. Please contact your administrator.")
		default:
			response.InternalError(c, "")
		}
		return
	}

	response.OK(c, "Login successful", result)
}

// Logout handles POST /api/v1/admin/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	if err := h.authService.Logout(c.Request.Context(), userID, username, c.ClientIP()); err != nil {
		response.InternalError(c, "")
		return
	}

	response.OK(c, "Logged out successfully", nil)
}

// RefreshToken handles POST /api/v1/admin/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Refresh token is required")
		return
	}

	result, err := h.authService.RefreshToken(c.Request.Context(), service.RefreshInput{
		RefreshToken: req.RefreshToken,
		IPAddress:    c.ClientIP(),
	})

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			response.Unauthorized(c, "Invalid or expired refresh token. Please log in again.")
		case errors.Is(err, service.ErrAccountInactive):
			response.Forbidden(c, "Your account has been deactivated.")
		default:
			response.InternalError(c, "")
		}
		return
	}

	response.OK(c, "Token refreshed successfully", result)
}

// Me handles GET /api/v1/admin/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)

	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "")
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Success: true,
		Message: "User profile retrieved",
		Data:    user,
	})
}
