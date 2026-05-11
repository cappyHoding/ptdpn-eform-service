package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
)

type WebhookHandler struct {
	contractService   service.ContractService
	contractRepo      repository.ContractRepository
	vidaWebhookSecret string
	log               *logger.Logger
}

type directSignWebhookData struct {
	EnvelopeID     string `json:"envelope_id"`
	RecipientEmail string `json:"recipient_email,omitempty"`
	Status         string `json:"status"` // "PENDING" | "SIGNED" | "COMPLETED" | "EXPIRED" | "DECLINED"
	UpdateAt       string `json:"update_at"`
}

func NewWebhookHandler(
	contractService service.ContractService,
	contractRepo repository.ContractRepository,
	vidaWebhookSecret string,
	log *logger.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		contractService:   contractService,
		contractRepo:      contractRepo,
		vidaWebhookSecret: vidaWebhookSecret,
		log:               log,
	}
}

// vidaWebhookPayload supports both Direct Sign and PoA legacy formats.
type vidaWebhookPayload struct {
	// Direct Sign fields (new format)
	EventType  string                 `json:"event_type"` // "ENVELOPE_SIGNED" | "ENVELOPE_EXPIRED" | "ENVELOPE_DECLINED"
	EnvelopeID string                 `json:"envelope_id"`
	Data       *directSignWebhookData `json:"data"`

	// PoA legacy fields
	EventID       string `json:"eventId"`
	LegacyType    string `json:"eventType"` // "document.signed" | "document.expired"
	TransactionID string `json:"transactionId"`
	SignedDoc     string `json:"signedDoc"`
	SignedAt      string `json:"signedAt"`
	Message       string `json:"message"`
	Status        string `json:"status"`
}

func (p *vidaWebhookPayload) normalizedEventType() string {
	// Direct Sign: cek status di dalam data untuk determine action
	if p.EventType == "envelope_status_changed" && p.Data != nil {
		switch p.Data.Status {
		case "COMPLETED":
			return "ENVELOPE_SIGNED"
		case "EXPIRED":
			return "ENVELOPE_EXPIRED"
		case "DECLINED":
			return "ENVELOPE_DECLINED"
		default:
			return "ENVELOPE_PENDING" // PENDING — tidak perlu diproses
		}
	}
	if p.EventType == "recipient_status_changed" && p.Data != nil {
		if p.Data.Status == "SIGNED" {
			return "RECIPIENT_SIGNED" // informational only
		}
		return "RECIPIENT_" + p.Data.Status
	}

	// PoA legacy
	switch p.LegacyType {
	case "document.signed":
		return "ENVELOPE_SIGNED"
	case "document.expired":
		return "ENVELOPE_EXPIRED"
	case "document.failed":
		return "ENVELOPE_DECLINED"
	default:
		return p.LegacyType
	}
}

func (p *vidaWebhookPayload) transactionRef() string {
	// Direct Sign: envelope_id ada di nested data
	if p.Data != nil && p.Data.EnvelopeID != "" {
		return p.Data.EnvelopeID
	}
	// PoA legacy
	return p.TransactionID
}

func (p *vidaWebhookPayload) idempotencyKey() string {
	if p.EventID != "" {
		return p.EventID
	}
	// Direct Sign: gabungkan envelope_id + event_type + status
	if p.Data != nil && p.Data.EnvelopeID != "" {
		return fmt.Sprintf("%s:%s:%s", p.Data.EnvelopeID, p.EventType, p.Data.Status)
	}
	return ""
}

func (h *WebhookHandler) HandleVida(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error("Failed to read webhook body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	// Verifikasi — support Direct Sign (plain header) + PoA (HMAC)
	if h.vidaWebhookSecret != "" && !h.verifyRequest(rawBody, c) {
		h.log.Warn("Invalid webhook signature", zap.String("ip", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var payload vidaWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		h.log.Error("Failed to parse webhook payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	eventType := payload.normalizedEventType()
	txRef := payload.transactionRef()

	h.log.Info("Webhook received",
		zap.String("event_type", eventType),
		zap.String("transaction_ref", txRef),
	)

	// Idempotency check
	if key := payload.idempotencyKey(); key != "" {
		if _, err := h.contractRepo.FindWebhookByVidaEventID(c.Request.Context(), key); err == nil {
			h.log.Info("Duplicate webhook, skipping", zap.String("key", key))
			c.JSON(http.StatusOK, gin.H{"message": "already processed"})
			return
		}
	}

	// Persist event
	eventID := payload.idempotencyKey()
	if eventID == "" {
		eventID = uuid.New().String()
	}
	var payloadJSON model.JSON
	_ = json.Unmarshal(rawBody, &payloadJSON)

	webhookEvent := &model.WebhookEvent{
		ID:              uuid.New().String(),
		Source:          "vida",
		EventType:       eventType,
		VidaEventID:     func() *string { s := eventID; return &s }(),
		Payload:         payloadJSON,
		Processed:       false,
		ProcessAttempts: 1,
	}
	if err := h.contractRepo.CreateWebhookEvent(c.Request.Context(), webhookEvent); err != nil {
		h.log.Error("Failed to save webhook event", zap.Error(err))
	}

	// Route by event type
	// Route by normalized event type
	var processErr error
	switch eventType {
	case "ENVELOPE_SIGNED":
		h.log.Info("Processing envelope signed",
			zap.String("envelope_id", txRef),
		)
		processErr = h.contractService.CompleteContract(
			c.Request.Context(), txRef, payload.SignedDoc,
		)
	case "ENVELOPE_EXPIRED", "ENVELOPE_DECLINED":
		reason := payload.Message
		if reason == "" {
			reason = eventType
		}
		processErr = h.contractService.FailContract(
			c.Request.Context(), txRef, reason,
		)
	case "RECIPIENT_SIGNED", "ENVELOPE_PENDING":
		// Informational only — log saja, tidak perlu action
		h.log.Info("Webhook informational event",
			zap.String("event_type", eventType),
			zap.String("envelope_id", txRef),
		)
		c.JSON(http.StatusOK, gin.H{"message": "acknowledged"})
		return
	default:
		h.log.Warn("Unknown webhook event type",
			zap.String("type", eventType),
			zap.String("raw_event_type", payload.EventType),
		)
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("event type %s not handled", eventType)})
		return
	}

	// Mark processed
	now := time.Now()
	if processErr != nil {
		h.log.Error("Webhook processing failed",
			zap.String("event_type", eventType),
			zap.String("tx_ref", txRef),
			zap.Error(processErr),
		)
		errMsg := processErr.Error()
		webhookEvent.ErrorMessage = &errMsg
		webhookEvent.Processed = false
	} else {
		webhookEvent.Processed = true
		webhookEvent.ProcessedAt = &now
	}
	_ = h.contractRepo.UpdateWebhookEvent(c.Request.Context(), webhookEvent)

	if processErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "processed"})
}

// verifyRequest mendukung dua metode:
// 1. Plain header secret (Direct Sign: X-VIDA-Webhook-Secret)
// 2. HMAC-SHA256 (PoA legacy: X-VIDA-Signature)
func (h *WebhookHandler) verifyRequest(body []byte, c *gin.Context) bool {
	if secret := c.GetHeader("X-VIDA-Webhook-Secret"); secret != "" {
		return hmac.Equal([]byte(secret), []byte(h.vidaWebhookSecret))
	}
	sig := c.GetHeader("X-VIDA-Signature")
	if strings.HasPrefix(sig, "sha256=") {
		received := strings.TrimPrefix(sig, "sha256=")
		mac := hmac.New(sha256.New, []byte(h.vidaWebhookSecret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(received), []byte(expected))
	}
	return false
}

// verifySignature kept for backward compatibility
func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	received := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(h.vidaWebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(received), []byte(expected))
}
