package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/cappyHoding/ptdpn-eform-service/internal/api/middleware"
	"github.com/cappyHoding/ptdpn-eform-service/internal/service"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// AdminHandler handles all internal staff dashboard endpoints.
type AdminHandler struct {
	adminService service.AdminService
}

func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type reviewActionRequest struct {
	Notes string `json:"notes"`
}

type rejectRequest struct {
	Notes string `json:"notes" binding:"required"`
}

type createUserRequest struct {
	Username string `json:"username"  binding:"required,min=3,max=50"`
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email"     binding:"required,email"`
	Password string `json:"password"  binding:"required,min=8"`
	RoleID   uint8  `json:"role_id"   binding:"required,oneof=1 2 3"`
}

type updateUserRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"    binding:"omitempty,email"`
	RoleID   *uint8 `json:"role_id"  binding:"omitempty,oneof=1 2 3"`
}

type updateConfigRequest struct {
	Value string `json:"value" binding:"required"`
}

// ─── Application Management ───────────────────────────────────────────────────

func (h *AdminHandler) ListApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.adminService.ListApplications(c.Request.Context(), service.ListApplicationsInput{
		Page:        page,
		PerPage:     perPage,
		Status:      c.Query("status"),
		ProductType: c.Query("product_type"),
		Search:      c.Query("search"),
	})
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Applications retrieved", result)
}

func (h *AdminHandler) GetApplicationDetail(c *gin.Context) {
	app, err := h.adminService.GetApplicationDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			response.NotFound(c, "Application not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Application detail retrieved", app)
}

func (h *AdminHandler) GetApplicationTimeline(c *gin.Context) {
	actions, err := h.adminService.GetApplicationTimeline(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			response.NotFound(c, "Application not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Timeline retrieved", gin.H{"timeline": actions})
}

// ─── Maker-Checker Workflow ───────────────────────────────────────────────────

func (h *AdminHandler) OpenApplication(c *gin.Context) {
	var req reviewActionRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.adminService.OpenApplication(c.Request.Context(), c.Param("id"), buildReview(c, req.Notes)); err != nil {
		handleAdminError(c, err)
		return
	}
	response.OK(c, "Application opened for review", gin.H{"status": "IN_REVIEW"})
}

func (h *AdminHandler) RecommendApplication(c *gin.Context) {
	var req reviewActionRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.adminService.RecommendApplication(c.Request.Context(), c.Param("id"), buildReview(c, req.Notes)); err != nil {
		handleAdminError(c, err)
		return
	}
	response.OK(c, "Application recommended for approval", gin.H{"status": "RECOMMENDED"})
}

func (h *AdminHandler) ApproveApplication(c *gin.Context) {
	var req reviewActionRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.adminService.ApproveApplication(c.Request.Context(), c.Param("id"), buildReview(c, req.Notes)); err != nil {
		handleAdminError(c, err)
		return
	}
	response.OK(c, "Application approved", gin.H{"status": "APPROVED"})
}

func (h *AdminHandler) RejectApplication(c *gin.Context) {
	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Rejection reason (notes) is required")
		return
	}
	if err := h.adminService.RejectApplication(c.Request.Context(), c.Param("id"), buildReview(c, req.Notes)); err != nil {
		handleAdminError(c, err)
		return
	}
	response.OK(c, "Application rejected", gin.H{"status": "REJECTED"})
}

func (h *AdminHandler) AddNote(c *gin.Context) {
	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Note content is required")
		return
	}
	if err := h.adminService.AddNote(c.Request.Context(), c.Param("id"), buildReview(c, req.Notes)); err != nil {
		handleAdminError(c, err)
		return
	}
	response.OK(c, "Note added", nil)
}

// ─── User Management ──────────────────────────────────────────────────────────

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	input := service.ListUsersInput{Page: page, PerPage: perPage, Search: c.Query("search")}
	if roleStr := c.Query("role_id"); roleStr != "" {
		if roleID, err := strconv.ParseUint(roleStr, 10, 8); err == nil {
			r := uint8(roleID)
			input.RoleID = &r
		}
	}
	if activeStr := c.Query("is_active"); activeStr != "" {
		active := activeStr == "true" || activeStr == "1"
		input.IsActive = &active
	}
	result, err := h.adminService.ListUsers(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Users retrieved", result)
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid user data: "+err.Error())
		return
	}
	user, err := h.adminService.CreateUser(c.Request.Context(), service.CreateUserInput{
		Username:  req.Username,
		FullName:  req.FullName,
		Email:     req.Email,
		Password:  req.Password,
		RoleID:    req.RoleID,
		CreatedBy: middleware.GetUserID(c),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameExists):
			response.Conflict(c, "Username already exists")
		case errors.Is(err, service.ErrEmailExists):
			response.Conflict(c, "Email already exists")
		default:
			response.InternalError(c, "")
		}
		return
	}
	c.JSON(http.StatusCreated, response.Response{Success: true, Message: "User created successfully", Data: user})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	user, err := h.adminService.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "User retrieved", user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid data: "+err.Error())
		return
	}
	user, err := h.adminService.UpdateUser(c.Request.Context(), c.Param("id"), service.UpdateUserInput{
		FullName: req.FullName,
		Email:    req.Email,
		RoleID:   req.RoleID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, "User not found")
		case errors.Is(err, service.ErrEmailExists):
			response.Conflict(c, "Email already exists")
		default:
			response.InternalError(c, "")
		}
		return
	}
	response.OK(c, "User updated", user)
}

func (h *AdminHandler) DeactivateUser(c *gin.Context) {
	if err := h.adminService.DeactivateUser(c.Request.Context(), c.Param("id"), middleware.GetUserID(c)); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "User deactivated", nil)
}

func (h *AdminHandler) ReactivateUser(c *gin.Context) {
	if err := h.adminService.ReactivateUser(c.Request.Context(), c.Param("id"), middleware.GetUserID(c)); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "User reactivated", nil)
}

// ─── System Config ────────────────────────────────────────────────────────────

func (h *AdminHandler) ListConfig(c *gin.Context) {
	configs, err := h.adminService.ListConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Configuration retrieved", gin.H{"configs": configs})
}

func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Value is required")
		return
	}
	cfg, err := h.adminService.UpdateConfig(c.Request.Context(), c.Param("key"), req.Value, middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrConfigNotFound) {
			response.NotFound(c, "Config key not found")
			return
		}
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Config updated", cfg)
}

// ─── Audit Logs ───────────────────────────────────────────────────────────────

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	result, err := h.adminService.ListAuditLogs(c.Request.Context(), service.ListAuditLogsInput{
		Page:       page,
		PerPage:    perPage,
		Action:     c.Query("action"),
		ActorID:    c.Query("actor_id"),
		EntityID:   c.Query("entity_id"),
		EntityType: c.Query("entity_type"),
	})
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Audit logs retrieved", result)
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	stats, err := h.adminService.GetDashboardStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Dashboard stats retrieved", gin.H{"stats": stats})
}

// ─── Private Helpers ──────────────────────────────────────────────────────────

func buildReview(c *gin.Context, notes string) service.ReviewInput {
	return service.ReviewInput{
		ActorID:       middleware.GetUserID(c),
		ActorUsername: middleware.GetUsername(c),
		ActorRole:     middleware.GetRole(c),
		Notes:         notes,
	}
}

func handleAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApplicationNotFound):
		response.NotFound(c, "Application not found")
	case errors.Is(err, service.ErrInvalidStatusChange):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrForbiddenAction):
		response.Forbidden(c, "You do not have permission for this action")
	case errors.Is(err, service.ErrAlreadySubmitted):
		response.BadRequest(c, "Application already submitted")
	default:
		response.InternalError(c, "")
	}
}
