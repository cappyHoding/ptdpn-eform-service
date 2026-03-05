package repository

import (
	"context"
	"fmt"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

// AuditRepository defines the interface for writing and reading audit logs.
// Write operations are append-only — no updates, no deletes.
type AuditRepository interface {
	Write(ctx context.Context, entry *model.AuditLog) error
	List(ctx context.Context, params ListAuditParams) ([]model.AuditLog, int64, error)
}

// ListAuditParams holds filter/pagination options for audit log queries.
type ListAuditParams struct {
	Page       int
	PerPage    int
	ActorID    string
	EntityType string
	EntityID   string
	Action     string
}

// auditRepository is the concrete MySQL implementation.
type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

// Write appends a new entry to the audit log.
// This is the only mutation allowed — no Update, no Delete.
func (r *auditRepository) Write(ctx context.Context, entry *model.AuditLog) error {
	result := r.db.WithContext(ctx).Create(entry)
	if result.Error != nil {
		// Audit log failures should be logged but not crash the request.
		// We return the error so the caller can decide how to handle it.
		return fmt.Errorf("failed to write audit log: %w", result.Error)
	}
	return nil
}

// List returns a paginated, filtered view of the audit log for the dashboard.
func (r *auditRepository) List(ctx context.Context, params ListAuditParams) ([]model.AuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AuditLog{})

	if params.ActorID != "" {
		query = query.Where("actor_id = ?", params.ActorID)
	}
	if params.EntityType != "" {
		query = query.Where("entity_type = ?", params.EntityType)
	}
	if params.EntityID != "" {
		query = query.Where("entity_id = ?", params.EntityID)
	}
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("audit log count failed: %w", err)
	}

	offset := (params.Page - 1) * params.PerPage
	var entries []model.AuditLog
	result := query.
		Order("created_at DESC").
		Limit(params.PerPage).
		Offset(offset).
		Find(&entries)

	if result.Error != nil {
		return nil, 0, fmt.Errorf("audit log list query failed: %w", result.Error)
	}

	return entries, total, nil
}
