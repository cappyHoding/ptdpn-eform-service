package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"gorm.io/gorm"
)

// ContractRepository handles contract_documents and webhook_events tables.
type ContractRepository interface {
	CreateContract(ctx context.Context, doc *model.ContractDocument) error
	FindContractByAppID(ctx context.Context, appID string) (*model.ContractDocument, error)
	FindContractBySignTrxID(ctx context.Context, signTrxID string) (*model.ContractDocument, error)
	UpdateContract(ctx context.Context, doc *model.ContractDocument) error
	// AcceptESignTOS menyimpan flag persetujuan TOS eSign nasabah ke contract_documents.
	AcceptESignTOS(ctx context.Context, appID string, ip string) error
	CreateWebhookEvent(ctx context.Context, event *model.WebhookEvent) error
	FindWebhookByVidaEventID(ctx context.Context, vidaEventID string) (*model.WebhookEvent, error)
	UpdateWebhookEvent(ctx context.Context, event *model.WebhookEvent) error
}

type contractRepository struct {
	db *gorm.DB
}

func NewContractRepository(db *gorm.DB) ContractRepository {
	return &contractRepository{db: db}
}

func (r *contractRepository) CreateContract(ctx context.Context, doc *model.ContractDocument) error {
	if err := r.db.WithContext(ctx).Create(doc).Error; err != nil {
		return fmt.Errorf("create contract failed: %w", err)
	}
	return nil
}

func (r *contractRepository) FindContractByAppID(ctx context.Context, appID string) (*model.ContractDocument, error) {
	var doc model.ContractDocument
	err := r.db.WithContext(ctx).
		Where("application_id = ? AND deleted_at IS NULL", appID).
		First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find contract by app_id failed: %w", err)
	}
	return &doc, nil
}

func (r *contractRepository) FindContractBySignTrxID(ctx context.Context, signTrxID string) (*model.ContractDocument, error) {
	var doc model.ContractDocument
	err := r.db.WithContext(ctx).
		Where("vida_sign_request_id = ? AND deleted_at IS NULL", signTrxID).
		First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find contract by sign trx_id failed: %w", err)
	}
	return &doc, nil
}

func (r *contractRepository) UpdateContract(ctx context.Context, doc *model.ContractDocument) error {
	if err := r.db.WithContext(ctx).Save(doc).Error; err != nil {
		return fmt.Errorf("update contract failed: %w", err)
	}
	return nil
}

func (r *contractRepository) AcceptESignTOS(ctx context.Context, appID string, ip string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.ContractDocument{}).
		Where("application_id = ? AND deleted_at IS NULL", appID).
		Updates(map[string]interface{}{
			"esign_tos_accepted":    true,
			"esign_tos_accepted_at": now,
			"esign_tos_ip":          ip,
		})
	if result.Error != nil {
		return fmt.Errorf("accept esign tos failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}


func (r *contractRepository) CreateWebhookEvent(ctx context.Context, event *model.WebhookEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create webhook event failed: %w", err)
	}
	return nil
}

func (r *contractRepository) FindWebhookByVidaEventID(ctx context.Context, vidaEventID string) (*model.WebhookEvent, error) {
	var event model.WebhookEvent
	err := r.db.WithContext(ctx).
		Where("vida_event_id = ?", vidaEventID).
		First(&event).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find webhook event failed: %w", err)
	}
	return &event, nil
}

func (r *contractRepository) UpdateWebhookEvent(ctx context.Context, event *model.WebhookEvent) error {
	if err := r.db.WithContext(ctx).Save(event).Error; err != nil {
		return fmt.Errorf("update webhook event failed: %w", err)
	}
	return nil
}
