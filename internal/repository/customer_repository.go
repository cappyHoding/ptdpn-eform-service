package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

// CustomerRepository defines database operations for customers.
type CustomerRepository interface {
	Create(ctx context.Context, customer *model.Customer) error
	FindByID(ctx context.Context, id string) (*model.Customer, error)
	FindByNIK(ctx context.Context, nik string) (*model.Customer, error)
	Update(ctx context.Context, customer *model.Customer) error
}

type customerRepository struct {
	db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) Create(ctx context.Context, customer *model.Customer) error {
	if err := r.db.WithContext(ctx).Create(customer).Error; err != nil {
		return fmt.Errorf("create customer failed: %w", err)
	}
	return nil
}

func (r *customerRepository) FindByID(ctx context.Context, id string) (*model.Customer, error) {
	var customer model.Customer
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("find customer by id failed: %w", err)
	}
	return &customer, nil
}

// FindByNIK checks if a customer with this NIK already exists.
// Used to prevent duplicate applications for the same identity.
func (r *customerRepository) FindByNIK(ctx context.Context, nik string) (*model.Customer, error) {
	var customer model.Customer
	err := r.db.WithContext(ctx).
		Where("nik = ? AND deleted_at IS NULL", nik).
		First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("find customer by NIK failed: %w", err)
	}
	return &customer, nil
}

func (r *customerRepository) Update(ctx context.Context, customer *model.Customer) error {
	if err := r.db.WithContext(ctx).Save(customer).Error; err != nil {
		return fmt.Errorf("update customer failed: %w", err)
	}
	return nil
}
