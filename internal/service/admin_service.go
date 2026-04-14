package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/crypto"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrConfigNotFound      = errors.New("config key not found")
	ErrInvalidStatusChange = errors.New("invalid status transition")
	ErrForbiddenAction     = errors.New("you do not have permission for this action")
	ErrUsernameExists      = errors.New("username already exists")
	ErrEmailExists         = errors.New("email already exists")
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type ListApplicationsInput struct {
	Page        int
	PerPage     int
	Status      string
	ProductType string
	Search      string
}

type ListApplicationsOutput struct {
	Applications []model.Application `json:"applications"`
	Total        int64               `json:"total"`
	Page         int                 `json:"page"`
	PerPage      int                 `json:"per_page"`
	TotalPages   int                 `json:"total_pages"`
}

type ReviewInput struct {
	ActorID       string
	ActorUsername string
	ActorRole     string
	Notes         string
}

type ListUsersInput struct {
	Page     int
	PerPage  int
	RoleID   *uint8
	IsActive *bool
	Search   string
}

type ListUsersOutput struct {
	Users      []model.InternalUser `json:"users"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PerPage    int                  `json:"per_page"`
	TotalPages int                  `json:"total_pages"`
}

type CreateUserInput struct {
	Username  string
	FullName  string
	Email     string
	Password  string
	RoleID    uint8
	CreatedBy string
}

type UpdateUserInput struct {
	FullName string
	Email    string
	RoleID   *uint8
}

type ListAuditLogsInput struct {
	Page       int
	PerPage    int
	Action     string
	ActorID    string
	EntityID   string
	EntityType string
}

type ListAuditLogsOutput struct {
	Logs       []model.AuditLog `json:"logs"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalPages int              `json:"total_pages"`
}

// ─── Interface ────────────────────────────────────────────────────────────────

type AdminService interface {
	// Application management
	ListApplications(ctx context.Context, input ListApplicationsInput) (*ListApplicationsOutput, error)
	GetApplicationDetail(ctx context.Context, appID string) (*model.Application, error)
	GetApplicationTimeline(ctx context.Context, appID string) ([]model.ReviewAction, error)

	// Maker-checker workflow
	OpenApplication(ctx context.Context, appID string, review ReviewInput) error
	RecommendApplication(ctx context.Context, appID string, review ReviewInput) error
	ApproveApplication(ctx context.Context, appID string, review ReviewInput) error
	RejectApplication(ctx context.Context, appID string, review ReviewInput) error
	AddNote(ctx context.Context, appID string, review ReviewInput) error

	// User management
	ListUsers(ctx context.Context, input ListUsersInput) (*ListUsersOutput, error)
	GetUser(ctx context.Context, userID string) (*model.InternalUser, error)
	CreateUser(ctx context.Context, input CreateUserInput) (*model.InternalUser, error)
	UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (*model.InternalUser, error)
	DeactivateUser(ctx context.Context, userID string, actorID string) error
	ReactivateUser(ctx context.Context, userID string, actorID string) error

	// System config
	ListConfig(ctx context.Context) ([]model.SystemConfig, error)
	UpdateConfig(ctx context.Context, key, value, updatedBy string) (*model.SystemConfig, error)

	// Audit logs
	ListAuditLogs(ctx context.Context, input ListAuditLogsInput) (*ListAuditLogsOutput, error)

	// Dashboard
	GetDashboardStats(ctx context.Context) (map[string]int64, error)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type adminService struct {
	appRepo     repository.ApplicationRepository
	userRepo    repository.UserRepository
	auditRepo   repository.AuditRepository
	configRepo  repository.ConfigRepository
	contractSvc ContractService
	notifSvc    NotificationService
	log         *logger.Logger
}

func NewAdminService(
	appRepo repository.ApplicationRepository,
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	configRepo repository.ConfigRepository,
	contractSvc ContractService,
	notifSvc NotificationService,
	log *logger.Logger,
) AdminService {
	return &adminService{
		appRepo:     appRepo,
		userRepo:    userRepo,
		auditRepo:   auditRepo,
		configRepo:  configRepo,
		contractSvc: contractSvc,
		notifSvc:    notifSvc,
		log:         log,
	}
}

// ── Application Management ────────────────────────────────────────────────────

func (s *adminService) ListApplications(ctx context.Context, input ListApplicationsInput) (*ListApplicationsOutput, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PerPage <= 0 || input.PerPage > 100 {
		input.PerPage = 20
	}

	apps, total, err := s.appRepo.List(ctx, repository.ListApplicationsParams{
		Page:        input.Page,
		PerPage:     input.PerPage,
		Status:      input.Status,
		ProductType: input.ProductType,
		Search:      input.Search,
	})
	if err != nil {
		return nil, fmt.Errorf("list applications failed: %w", err)
	}

	totalPages := int(total) / input.PerPage
	if int(total)%input.PerPage > 0 {
		totalPages++
	}

	return &ListApplicationsOutput{
		Applications: apps,
		Total:        total,
		Page:         input.Page,
		PerPage:      input.PerPage,
		TotalPages:   totalPages,
	}, nil
}

func (s *adminService) GetApplicationDetail(ctx context.Context, appID string) (*model.Application, error) {
	app, err := s.appRepo.FindByIDWithDetails(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("get application detail failed: %w", err)
	}
	return app, nil
}

func (s *adminService) GetApplicationTimeline(ctx context.Context, appID string) ([]model.ReviewAction, error) {
	// Verify app exists first
	if _, err := s.appRepo.FindByID(ctx, appID); err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, err
	}

	actions, err := s.appRepo.FindReviewActionsByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("get timeline failed: %w", err)
	}
	return actions, nil
}

// ── Maker-Checker Workflow ────────────────────────────────────────────────────

// OpenApplication: PENDING_REVIEW → IN_REVIEW
// Only operators and admins can open. This is the "maker" picking up a case.
func (s *adminService) OpenApplication(ctx context.Context, appID string, review ReviewInput) error {
	app, err := s.appRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if app.Status != model.StatusPendingReview {
		return fmt.Errorf("%w: application must be in PENDING_REVIEW to open (current: %s)",
			ErrInvalidStatusChange, app.Status)
	}

	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusInReview); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return s.recordReviewAction(ctx, appID, review, "OPENED",
		fmt.Sprintf("Application opened for review by %s", review.ActorUsername))
}

// RecommendApplication: IN_REVIEW → RECOMMENDED
// Operator (maker) recommends for supervisor approval.
func (s *adminService) RecommendApplication(ctx context.Context, appID string, review ReviewInput) error {
	app, err := s.appRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if app.Status != model.StatusInReview {
		return fmt.Errorf("%w: application must be IN_REVIEW to recommend (current: %s)",
			ErrInvalidStatusChange, app.Status)
	}

	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusRecommended); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return s.recordReviewAction(ctx, appID, review, "RECOMMENDED",
		fmt.Sprintf("Application recommended for approval by operator %s", review.ActorUsername))
}

// ApproveApplication: RECOMMENDED → APPROVED
// Supervisor (checker) approves. Only supervisor/admin can call this.
func (s *adminService) ApproveApplication(ctx context.Context, appID string, review ReviewInput) error {
	app, err := s.appRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if app.Status != model.StatusRecommended {
		return fmt.Errorf("%w: application must be RECOMMENDED to approve (current: %s)",
			ErrInvalidStatusChange, app.Status)
	}

	// Hanya supervisor dan admin yang boleh approve
	if review.ActorRole == "operator" {
		return ErrForbiddenAction
	}

	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusApproved); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if err := s.recordReviewAction(ctx, appID, review, "APPROVED",
		fmt.Sprintf("Application approved by %s", review.ActorUsername)); err != nil {
		return err
	}

	// ── Inisiasi contract flow (best-effort — log error tapi jangan gagalkan approve) ──
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("Approval goroutine panicked",
					zap.String("app_id", appID), zap.Any("panic", r))
			}
		}()

		bgCtx := context.Background()

		appDetail, err := s.appRepo.FindByIDWithDetails(bgCtx, appID)
		if err != nil || appDetail.Customer.Email == nil {
			s.log.Warn("Cannot send approval emails — customer not found or email empty",
				zap.String("app_id", appID))
			return
		}

		customerName := "Nasabah"
		if appDetail.Customer.FullName != nil {
			customerName = *appDetail.Customer.FullName
		}
		customerEmail := *appDetail.Customer.Email
		productName := string(appDetail.ProductType)

		// 1. Kirim email approval
		if err := s.notifSvc.SendApprovalNotice(
			bgCtx, appID, customerEmail, customerName, productName,
		); err != nil {
			s.log.Warn("Approval email failed", zap.Error(err))
		}

		// 2. Jeda — hindari rate limit Mailtrap (1 email/detik)
		time.Sleep(5 * time.Second)

		// 3. Inisiasi contract (generate PDF mock + kirim eSign email)
		if err := s.contractSvc.InitiateContract(bgCtx, appID, review.ActorID); err != nil {
			s.log.Error("Contract initiation failed",
				zap.String("app_id", appID),
				zap.String("actor", review.ActorUsername),
				zap.Error(err),
			)
		}
	}()

	return nil
}

// RejectApplication: any active status → REJECTED
// Supervisor/admin can reject at any stage.
func (s *adminService) RejectApplication(ctx context.Context, appID string, review ReviewInput) error {
	if review.ActorRole != "supervisor" && review.ActorRole != "admin" {
		return ErrForbiddenAction
	}

	app, err := s.appRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	// Can reject from any active status
	rejectableStatuses := map[model.ApplicationStatus]bool{
		model.StatusPendingReview: true,
		model.StatusInReview:      true,
		model.StatusRecommended:   true,
	}
	if !rejectableStatuses[app.Status] {
		return fmt.Errorf("%w: cannot reject application in status %s",
			ErrInvalidStatusChange, app.Status)
	}

	if review.Notes == "" {
		return fmt.Errorf("rejection reason (notes) is required")
	}

	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusRejected); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:     "internal_user",
		ActorID:       &review.ActorID,
		ActorUsername: &review.ActorUsername,
		ActorRole:     &review.ActorRole,
		Action:        "APP_REJECTED",
		EntityType:    strPtrIfNotEmpty("application"),
		EntityID:      strPtrIfNotEmpty(appID),
		Description:   strPtrIfNotEmpty(fmt.Sprintf("Application rejected: %s", review.Notes)),
		NewValue:      model.JSON{"status": "REJECTED", "reason": review.Notes},
	})

	go func() {
		app, err := s.appRepo.FindByIDWithDetails(context.Background(), appID)
		if err != nil || app.Customer.Email == nil {
			return
		}
		customerName := "Nasabah"
		if app.Customer.FullName != nil {
			customerName = *app.Customer.FullName
		}
		productName := string(app.ProductType)
		reason := review.Notes
		if reason == "" {
			reason = "Tidak memenuhi persyaratan yang ditetapkan"
		}
		if err := s.notifSvc.SendRejectionNotice(
			context.Background(),
			appID,
			*app.Customer.Email,
			customerName,
			productName,
			reason,
		); err != nil {
			s.log.Warn("Rejection email failed", zap.Error(err))
		}
	}()

	return s.recordReviewAction(ctx, appID, review, "REJECTED", review.Notes)
}

// AddNote: adds a note without changing status.
func (s *adminService) AddNote(ctx context.Context, appID string, review ReviewInput) error {
	if _, err := s.appRepo.FindByID(ctx, appID); err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if review.Notes == "" {
		return fmt.Errorf("note content is required")
	}

	return s.recordReviewAction(ctx, appID, review, "NOTE_ADDED", review.Notes)
}

// ── User Management ───────────────────────────────────────────────────────────

func (s *adminService) ListUsers(ctx context.Context, input ListUsersInput) (*ListUsersOutput, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PerPage <= 0 || input.PerPage > 100 {
		input.PerPage = 20
	}

	users, total, err := s.userRepo.List(ctx, repository.ListUsersParams{
		Page:     input.Page,
		PerPage:  input.PerPage,
		RoleID:   input.RoleID,
		IsActive: input.IsActive,
		Search:   input.Search,
	})
	if err != nil {
		return nil, fmt.Errorf("list users failed: %w", err)
	}

	totalPages := int(total) / input.PerPage
	if int(total)%input.PerPage > 0 {
		totalPages++
	}

	return &ListUsersOutput{
		Users:      users,
		Total:      total,
		Page:       input.Page,
		PerPage:    input.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (s *adminService) GetUser(ctx context.Context, userID string) (*model.InternalUser, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *adminService) CreateUser(ctx context.Context, input CreateUserInput) (*model.InternalUser, error) {
	// Check username uniqueness
	if _, err := s.userRepo.FindByUsername(ctx, input.Username); err == nil {
		return nil, ErrUsernameExists
	}

	// Check email uniqueness
	if _, err := s.userRepo.FindByEmail(ctx, input.Email); err == nil {
		return nil, ErrEmailExists
	}

	hashedPwd, err := crypto.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.InternalUser{
		ID:        uuid.New().String(),
		Username:  input.Username,
		FullName:  input.FullName,
		Email:     input.Email,
		Password:  hashedPwd,
		RoleID:    input.RoleID,
		IsActive:  true,
		CreatedBy: &input.CreatedBy,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user failed: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "internal_user",
		ActorID:     &input.CreatedBy,
		Action:      "USER_CREATED",
		EntityType:  strPtrIfNotEmpty("internal_user"),
		EntityID:    strPtrIfNotEmpty(user.ID),
		Description: strPtrIfNotEmpty(fmt.Sprintf("User %s created with role_id %d", user.Username, user.RoleID)),
		NewValue:    model.JSON{"username": user.Username, "role_id": user.RoleID},
	})

	s.log.Info("Internal user created",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username),
		zap.Uint8("role_id", user.RoleID),
	)

	// Re-fetch to include role association
	return s.userRepo.FindByID(ctx, user.ID)
}

func (s *adminService) UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (*model.InternalUser, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if input.FullName != "" {
		user.FullName = input.FullName
	}
	if input.Email != "" {
		// Check email uniqueness only if it changed
		if input.Email != user.Email {
			if _, err := s.userRepo.FindByEmail(ctx, input.Email); err == nil {
				return nil, ErrEmailExists
			}
		}
		user.Email = input.Email
	}
	if input.RoleID != nil {
		user.RoleID = *input.RoleID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user failed: %w", err)
	}

	return s.userRepo.FindByID(ctx, userID)
}

func (s *adminService) DeactivateUser(ctx context.Context, userID string, actorID string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	user.IsActive = false
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("deactivate user failed: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "internal_user",
		ActorID:     &actorID,
		Action:      "USER_DEACTIVATED",
		EntityType:  strPtrIfNotEmpty("internal_user"),
		EntityID:    strPtrIfNotEmpty(userID),
		Description: strPtrIfNotEmpty(fmt.Sprintf("User %s deactivated", user.Username)),
	})

	return nil
}

func (s *adminService) ReactivateUser(ctx context.Context, userID string, actorID string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	user.IsActive = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("reactivate user failed: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "internal_user",
		ActorID:     &actorID,
		Action:      "USER_REACTIVATED",
		EntityType:  strPtrIfNotEmpty("internal_user"),
		EntityID:    strPtrIfNotEmpty(userID),
		Description: strPtrIfNotEmpty(fmt.Sprintf("User %s reactivated", user.Username)),
	})

	return nil
}

// ── System Config ─────────────────────────────────────────────────────────────

func (s *adminService) ListConfig(ctx context.Context) ([]model.SystemConfig, error) {
	return s.configRepo.List(ctx)
}

func (s *adminService) UpdateConfig(ctx context.Context, key, value, updatedBy string) (*model.SystemConfig, error) {
	// Verify key exists
	existing, err := s.configRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	oldValue := existing.ConfigValue
	existing.ConfigValue = value
	existing.UpdatedBy = &updatedBy

	if err := s.configRepo.Upsert(ctx, existing); err != nil {
		return nil, fmt.Errorf("update config failed: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "internal_user",
		ActorID:     &updatedBy,
		Action:      "CONFIG_UPDATED",
		EntityType:  strPtrIfNotEmpty("system_config"),
		EntityID:    strPtrIfNotEmpty(key),
		Description: strPtrIfNotEmpty(fmt.Sprintf("Config %s updated", key)),
		OldValue:    model.JSON{"value": oldValue},
		NewValue:    model.JSON{"value": value},
	})

	return s.configRepo.FindByKey(ctx, key)
}

// ── Audit Logs ────────────────────────────────────────────────────────────────

func (s *adminService) ListAuditLogs(ctx context.Context, input ListAuditLogsInput) (*ListAuditLogsOutput, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PerPage <= 0 || input.PerPage > 200 {
		input.PerPage = 50
	}

	logs, total, err := s.auditRepo.List(ctx, repository.ListAuditParams{
		Page:       input.Page,
		PerPage:    input.PerPage,
		Action:     input.Action,
		ActorID:    input.ActorID,
		EntityID:   input.EntityID,
		EntityType: input.EntityType,
	})
	if err != nil {
		return nil, fmt.Errorf("list audit logs failed: %w", err)
	}

	totalPages := int(total) / input.PerPage
	if int(total)%input.PerPage > 0 {
		totalPages++
	}

	return &ListAuditLogsOutput{
		Logs:       logs,
		Total:      total,
		Page:       input.Page,
		PerPage:    input.PerPage,
		TotalPages: totalPages,
	}, nil
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (s *adminService) GetDashboardStats(ctx context.Context) (map[string]int64, error) {
	return s.appRepo.GetDashboardStats(ctx)
}

// ── Private Helpers ───────────────────────────────────────────────────────────

func (s *adminService) recordReviewAction(
	ctx context.Context,
	appID string,
	review ReviewInput,
	action string,
	description string,
) error {
	notes := review.Notes
	var notesPtr *string
	if notes != "" {
		notesPtr = &notes
	}

	ra := &model.ReviewAction{
		ID:            uuid.New().String(),
		ApplicationID: appID,
		ActorID:       review.ActorID,
		ActorUsername: review.ActorUsername,
		ActorRole:     review.ActorRole,
		Action:        action,
		Notes:         notesPtr,
	}

	if err := s.appRepo.CreateReviewAction(ctx, ra); err != nil {
		return fmt.Errorf("failed to record review action: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:     "internal_user",
		ActorID:       &review.ActorID,
		ActorUsername: &review.ActorUsername,
		ActorRole:     &review.ActorRole,
		Action:        "APP_" + action,
		EntityType:    strPtrIfNotEmpty("application"),
		EntityID:      strPtrIfNotEmpty(appID),
		Description:   strPtrIfNotEmpty(description),
	})

	return nil
}

func (s *adminService) writeAudit(ctx context.Context, entry *model.AuditLog) {
	entry.CreatedAt = time.Now()
	if err := s.auditRepo.Write(ctx, entry); err != nil {
		s.log.Error("Failed to write audit log",
			zap.String("action", entry.Action),
			zap.Error(err),
		)
	}
}
