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

// WebhookHandler handles incoming webhook calls from VIDA.
type WebhookHandler struct {
	contractService   service.ContractService
	contractRepo      repository.ContractRepository
	vidaWebhookSecret string
	log               *logger.Logger
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

// ─── VIDA Webhook Payload ─────────────────────────────────────────────────────

// vidaWebhookPayload represents the JSON body VIDA sends to our endpoint.
// Based on VIDA PoA eSign webhook documentation.
type vidaWebhookPayload struct {
	EventID       string `json:"eventId"`
	EventType     string `json:"eventType"` // "document.signed" | "document.expired" | "document.failed"
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`    // "COMPLETED" | "EXPIRED" | "FAILED"
	SignedDoc     string `json:"signedDoc"` // base64 signed PDF (only on document.signed)
	SignedAt      string `json:"signedAt"`
	Message       string `json:"message"`
}

// HandleVida handles POST /webhooks/vida
//
// Flow:
//  1. Verify HMAC-SHA256 signature from X-VIDA-Signature header
//  2. Record raw event to webhook_events table (idempotency)
//  3. Route to ContractService based on event type
//  4. Mark event as processed
func (h *WebhookHandler) HandleVida(c *gin.Context) {
	// ── 1. Read raw body (needed for HMAC verification) ───────────────────────
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error("Failed to read webhook body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	// ── 2. Verify HMAC signature ──────────────────────────────────────────────
	signature := c.GetHeader("X-VIDA-Signature")
	if h.vidaWebhookSecret != "" && !h.verifySignature(rawBody, signature) {
		h.log.Warn("Invalid webhook signature",
			zap.String("received", signature),
			zap.String("ip", c.ClientIP()),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// ── 3. Parse payload ──────────────────────────────────────────────────────
	var payload vidaWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		h.log.Error("Failed to parse webhook payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	h.log.Info("Webhook received",
		zap.String("event_type", payload.EventType),
		zap.String("transaction_id", payload.TransactionID),
		zap.String("status", payload.Status),
	)

	// ── 4. Idempotency check — skip if already processed ─────────────────────
	if payload.EventID != "" {
		if _, err := h.contractRepo.FindWebhookByVidaEventID(c.Request.Context(), payload.EventID); err == nil {
			// Already processed — return 200 so VIDA doesn't retry
			h.log.Info("Duplicate webhook, skipping", zap.String("event_id", payload.EventID))
			c.JSON(http.StatusOK, gin.H{"message": "already processed"})
			return
		}
	}

	// ── 5. Persist raw event ──────────────────────────────────────────────────
	eventID := payload.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	var payloadJSON model.JSON
	_ = json.Unmarshal(rawBody, &payloadJSON)

	webhookEvent := &model.WebhookEvent{
		ID:        uuid.New().String(),
		Source:    "vida",
		EventType: payload.EventType,
		VidaEventID: func() *string {
			s := eventID
			return &s
		}(),
		Payload:         payloadJSON,
		Processed:       false,
		ProcessAttempts: 1,
	}

	if err := h.contractRepo.CreateWebhookEvent(c.Request.Context(), webhookEvent); err != nil {
		h.log.Error("Failed to save webhook event", zap.Error(err))
		// Don't fail — still try to process
	}

	// ── 6. Route by event type ────────────────────────────────────────────────
	var processErr error
	switch payload.EventType {
	case "document.signed":
		processErr = h.contractService.CompleteContract(
			c.Request.Context(),
			payload.TransactionID,
			payload.SignedDoc,
		)
	case "document.expired", "document.failed":
		reason := payload.Message
		if reason == "" {
			reason = payload.EventType
		}
		processErr = h.contractService.FailContract(
			c.Request.Context(),
			payload.TransactionID,
			reason,
		)
	default:
		h.log.Warn("Unknown webhook event type", zap.String("type", payload.EventType))
		// Still return 200 — unknown events shouldn't cause retries
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("event type %s not handled", payload.EventType)})
		return
	}

	// ── 7. Mark event processed / failed ─────────────────────────────────────
	now := time.Now()
	if processErr != nil {
		h.log.Error("Webhook processing failed",
			zap.String("event_type", payload.EventType),
			zap.String("transaction_id", payload.TransactionID),
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
		// Return 500 so VIDA retries the webhook
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "processed"})
}

// verifySignature checks HMAC-SHA256 signature from VIDA.
// VIDA sends: X-VIDA-Signature: sha256=<hex_hash>
func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	received := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(h.vidaWebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(received), []byte(expected))
}
