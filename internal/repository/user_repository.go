// Package repository contains all database query logic.
// Repositories have ONE job: talk to the database.
// No business logic here — just clean, reusable query functions.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

// UserRepository defines the interface for user database operations.
// Using an interface means our service can be tested with a mock repository
// without needing a real database connection.
type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.InternalUser, error)
	FindByID(ctx context.Context, id string) (*model.InternalUser, error)
	FindByEmail(ctx context.Context, email string) (*model.InternalUser, error)
	Create(ctx context.Context, user *model.InternalUser) error
	Update(ctx context.Context, user *model.InternalUser) error
	List(ctx context.Context, params ListUsersParams) ([]model.InternalUser, int64, error)
}

// ListUsersParams holds filter/pagination options for listing users.
type ListUsersParams struct {
	Page     int
	PerPage  int
	RoleID   *uint8
	IsActive *bool
	Search   string // searches username and full_name
}

// userRepository is the concrete MySQL implementation of UserRepository.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository backed by the given GORM db.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// FindByUsername retrieves an active internal user by their username.
// Returns nil, ErrUserNotFound if not found or soft-deleted.
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.InternalUser, error) {
	var user model.InternalUser
	result := r.db.WithContext(ctx).
		Preload("Role"). // load the associated Role record
		Where("username = ? AND deleted_at IS NULL", username).
		First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("FindByUsername query failed: %w", result.Error)
	}

	return &user, nil
}

// FindByID retrieves an internal user by their UUID.
func (r *userRepository) FindByID(ctx context.Context, id string) (*model.InternalUser, error) {
	var user model.InternalUser
	result := r.db.WithContext(ctx).
		Preload("Role").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("FindByID query failed: %w", result.Error)
	}

	return &user, nil
}

// FindByEmail retrieves an internal user by their email address.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.InternalUser, error) {
	var user model.InternalUser
	result := r.db.WithContext(ctx).
		Preload("Role").
		Where("email = ? AND deleted_at IS NULL", email).
		First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("FindByEmail query failed: %w", result.Error)
	}

	return &user, nil
}

// Create inserts a new internal user record.
func (r *userRepository) Create(ctx context.Context, user *model.InternalUser) error {
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		return fmt.Errorf("Create user failed: %w", result.Error)
	}
	return nil
}

// Update saves changes to an existing internal user record.
// Only updates non-zero fields — use Select() to update specific fields.
func (r *userRepository) Update(ctx context.Context, user *model.InternalUser) error {
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return fmt.Errorf("Update user failed: %w", result.Error)
	}
	return nil
}

// List returns a paginated list of internal users with optional filters.
func (r *userRepository) List(ctx context.Context, params ListUsersParams) ([]model.InternalUser, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.InternalUser{}).
		Preload("Role").
		Where("deleted_at IS NULL")

	// Apply optional filters
	if params.RoleID != nil {
		query = query.Where("role_id = ?", *params.RoleID)
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}
	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("username LIKE ? OR full_name LIKE ?", search, search)
	}

	// Count total matching records (before pagination)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("List users count failed: %w", err)
	}

	// Apply pagination
	offset := (params.Page - 1) * params.PerPage
	var users []model.InternalUser
	result := query.
		Order("created_at DESC").
		Limit(params.PerPage).
		Offset(offset).
		Find(&users)

	if result.Error != nil {
		return nil, 0, fmt.Errorf("List users query failed: %w", result.Error)
	}

	return users, total, nil
}
