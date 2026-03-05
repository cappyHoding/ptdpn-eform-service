package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

// ApplicationRepository defines all database operations for applications.
type ApplicationRepository interface {
	Create(ctx context.Context, app *model.Application) error
	FindByID(ctx context.Context, id string) (*model.Application, error)
	FindByIDWithDetails(ctx context.Context, id string) (*model.Application, error)
	UpdateStatus(ctx context.Context, id string, status model.ApplicationStatus) error
	UpdateStep(ctx context.Context, id string, currentStep, lastCompleted uint8) error
	Update(ctx context.Context, app *model.Application) error

	// Product detail operations
	CreateSavingDetail(ctx context.Context, detail *model.SavingDetail) error
	CreateDepositDetail(ctx context.Context, detail *model.DepositDetail) error
	CreateLoanDetail(ctx context.Context, detail *model.LoanDetail) error

	// Disbursement
	UpsertDisbursement(ctx context.Context, data *model.DisbursementData) error

	// eKYC results
	UpsertOCRResult(ctx context.Context, result *model.OCRResult) error
	UpsertLivenessResult(ctx context.Context, result *model.LivenessResult) error

	// Session
	CreateSession(ctx context.Context, session *model.CustomerSession) error
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (*model.CustomerSession, error)
	RevokeSession(ctx context.Context, sessionID string) error

	// Admin list
	List(ctx context.Context, params ListApplicationsParams) ([]model.Application, int64, error)

	// Review actions (maker-checker trail)
	CreateReviewAction(ctx context.Context, action *model.ReviewAction) error
	FindReviewActionsByAppID(ctx context.Context, appID string) ([]model.ReviewAction, error)

	// Dashboard
	GetDashboardStats(ctx context.Context) (map[string]int64, error)
}

// ListApplicationsParams holds filters for the admin dashboard list.
type ListApplicationsParams struct {
	Page        int
	PerPage     int
	Status      string
	ProductType string
	Search      string // searches customer NIK or name
}

type applicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{db: db}
}

// ─── Application Core ─────────────────────────────────────────────────────────

func (r *applicationRepository) Create(ctx context.Context, app *model.Application) error {
	if err := r.db.WithContext(ctx).Create(app).Error; err != nil {
		return fmt.Errorf("create application failed: %w", err)
	}
	return nil
}

// FindByID loads an application without its associated records.
// Use for simple status checks and step validation.
func (r *applicationRepository) FindByID(ctx context.Context, id string) (*model.Application, error) {
	var app model.Application
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("find application failed: %w", err)
	}
	return &app, nil
}

// FindByIDWithDetails loads an application with ALL associated records.
// Use for the summary page (Step 7) and admin review dashboard.
// Uses GORM Preload to fetch related tables in separate queries.
func (r *applicationRepository) FindByIDWithDetails(ctx context.Context, id string) (*model.Application, error) {
	var app model.Application
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Preload("SavingDetail").
		Preload("DepositDetail").
		Preload("LoanDetail").
		Preload("CollateralItems").
		Preload("DisbursementData").
		Preload("OCRResult").
		Preload("LivenessResult").
		Preload("ContractDocument").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("find application with details failed: %w", err)
	}
	return &app, nil
}

// UpdateStatus changes the application's status in the state machine.
// Also updates the updated_at timestamp automatically via GORM.
func (r *applicationRepository) UpdateStatus(ctx context.Context, id string, status model.ApplicationStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.Application{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update application status failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrApplicationNotFound
	}
	return nil
}

// UpdateStep advances the customer's current step and last completed step.
func (r *applicationRepository) UpdateStep(ctx context.Context, id string, currentStep, lastCompleted uint8) error {
	result := r.db.WithContext(ctx).
		Model(&model.Application{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"current_step":        currentStep,
			"last_step_completed": lastCompleted,
		})
	if result.Error != nil {
		return fmt.Errorf("update application step failed: %w", result.Error)
	}
	return nil
}

func (r *applicationRepository) Update(ctx context.Context, app *model.Application) error {
	if err := r.db.WithContext(ctx).Save(app).Error; err != nil {
		return fmt.Errorf("update application failed: %w", err)
	}
	return nil
}

// ─── Product Details ──────────────────────────────────────────────────────────

func (r *applicationRepository) CreateSavingDetail(ctx context.Context, detail *model.SavingDetail) error {
	if err := r.db.WithContext(ctx).Create(detail).Error; err != nil {
		return fmt.Errorf("create saving detail failed: %w", err)
	}
	return nil
}

func (r *applicationRepository) CreateDepositDetail(ctx context.Context, detail *model.DepositDetail) error {
	if err := r.db.WithContext(ctx).Create(detail).Error; err != nil {
		return fmt.Errorf("create deposit detail failed: %w", err)
	}
	return nil
}

func (r *applicationRepository) CreateLoanDetail(ctx context.Context, detail *model.LoanDetail) error {
	if err := r.db.WithContext(ctx).Create(detail).Error; err != nil {
		return fmt.Errorf("create loan detail failed: %w", err)
	}
	return nil
}

// ─── Disbursement ─────────────────────────────────────────────────────────────

// UpsertDisbursement creates or updates disbursement data.
// Customers can update their bank account before final submission.
func (r *applicationRepository) UpsertDisbursement(ctx context.Context, data *model.DisbursementData) error {
	err := r.db.WithContext(ctx).
		Where("application_id = ?", data.ApplicationID).
		Assign(data).
		FirstOrCreate(data).Error
	if err != nil {
		return fmt.Errorf("upsert disbursement failed: %w", err)
	}
	return nil
}

// ─── eKYC Results ─────────────────────────────────────────────────────────────

// UpsertOCRResult creates or updates the OCR result.
// Customers may re-upload their KTP if the first scan fails.
func (r *applicationRepository) UpsertOCRResult(ctx context.Context, result *model.OCRResult) error {
	existing := &model.OCRResult{}
	err := r.db.WithContext(ctx).
		Where("application_id = ?", result.ApplicationID).
		First(existing).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("upsert OCR result lookup failed: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// First time — create
		if createErr := r.db.WithContext(ctx).Create(result).Error; createErr != nil {
			return fmt.Errorf("create OCR result failed: %w", createErr)
		}
	} else {
		// Already exists — update
		result.ID = existing.ID
		if saveErr := r.db.WithContext(ctx).Save(result).Error; saveErr != nil {
			return fmt.Errorf("update OCR result failed: %w", saveErr)
		}
	}
	return nil
}

// UpsertLivenessResult creates or updates the liveness result.
func (r *applicationRepository) UpsertLivenessResult(ctx context.Context, result *model.LivenessResult) error {
	existing := &model.LivenessResult{}
	err := r.db.WithContext(ctx).
		Where("application_id = ?", result.ApplicationID).
		First(existing).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("upsert liveness result lookup failed: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if createErr := r.db.WithContext(ctx).Create(result).Error; createErr != nil {
			return fmt.Errorf("create liveness result failed: %w", createErr)
		}
	} else {
		result.ID = existing.ID
		if saveErr := r.db.WithContext(ctx).Save(result).Error; saveErr != nil {
			return fmt.Errorf("update liveness result failed: %w", saveErr)
		}
	}
	return nil
}

// ─── Customer Sessions ────────────────────────────────────────────────────────

func (r *applicationRepository) CreateSession(ctx context.Context, session *model.CustomerSession) error {
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create session failed: %w", err)
	}
	return nil
}

// FindSessionByTokenHash looks up a session by the SHA-256 hash of the token.
// We store hashes, never raw tokens, so this is the only valid lookup method.
func (r *applicationRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (*model.CustomerSession, error) {
	var session model.CustomerSession
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND deleted_at IS NULL AND revoked_at IS NULL", tokenHash).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("find session failed: %w", err)
	}
	return &session, nil
}

// RevokeSession marks a session as revoked when a step is completed.
func (r *applicationRepository) RevokeSession(ctx context.Context, sessionID string) error {
	result := r.db.WithContext(ctx).
		Model(&model.CustomerSession{}).
		Where("id = ?", sessionID).
		Update("revoked_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return fmt.Errorf("revoke session failed: %w", result.Error)
	}
	return nil
}

// ─── Admin List ───────────────────────────────────────────────────────────────

func (r *applicationRepository) List(ctx context.Context, params ListApplicationsParams) ([]model.Application, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Application{}).
		Preload("Customer").
		Where("applications.deleted_at IS NULL")

	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.ProductType != "" {
		query = query.Where("product_type = ?", params.ProductType)
	}
	if params.Search != "" {
		// Join customers table to search by name or NIK
		query = query.
			Joins("LEFT JOIN customers ON customers.id = applications.customer_id").
			Where("customers.nik LIKE ? OR customers.full_name LIKE ?",
				"%"+params.Search+"%", "%"+params.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("list applications count failed: %w", err)
	}

	offset := (params.Page - 1) * params.PerPage
	var apps []model.Application
	err := query.
		Order("applications.created_at DESC").
		Limit(params.PerPage).
		Offset(offset).
		Find(&apps).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list applications query failed: %w", err)
	}

	return apps, total, nil
}

// ─── Review Actions ───────────────────────────────────────────────────────────

// CreateReviewAction inserts a new review action record (maker-checker trail).
func (r *applicationRepository) CreateReviewAction(ctx context.Context, action *model.ReviewAction) error {
	if err := r.db.WithContext(ctx).Create(action).Error; err != nil {
		return fmt.Errorf("create review action failed: %w", err)
	}
	return nil
}

// FindReviewActionsByAppID returns all review actions for an application,
// ordered chronologically for the timeline view.
func (r *applicationRepository) FindReviewActionsByAppID(ctx context.Context, appID string) ([]model.ReviewAction, error) {
	var actions []model.ReviewAction
	err := r.db.WithContext(ctx).
		Where("application_id = ?", appID).
		Order("created_at ASC").
		Find(&actions).Error
	if err != nil {
		return nil, fmt.Errorf("find review actions failed: %w", err)
	}
	return actions, nil
}

// GetDashboardStats returns counts grouped by status for the admin home screen.
func (r *applicationRepository) GetDashboardStats(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	err := r.db.WithContext(ctx).
		Model(&model.Application{}).
		Select("status, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("dashboard stats query failed: %w", err)
	}

	stats := make(map[string]int64)
	var total int64
	for _, row := range rows {
		stats[row.Status] = row.Count
		total += row.Count
	}
	stats["TOTAL"] = total
	return stats, nil
}

// Ensure new methods are declared in the interface — add them inline:
// (compile-time check via interface assertion in main.go)
