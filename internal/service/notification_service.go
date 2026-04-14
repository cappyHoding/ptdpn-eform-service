package service

import (
	"context"
	"fmt"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/email"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NotificationService kirim email ke nasabah dan catat ke notification_logs.
type NotificationService interface {
	SendSubmitConfirmation(ctx context.Context, appID, customerEmail, customerName, productName string) error
	SendRejectionNotice(ctx context.Context, appID, customerEmail, customerName, productName, reason string) error
	SendApprovalNotice(ctx context.Context, appID, customerEmail, customerName, productName string) error
	SendESignLink(ctx context.Context, appID, toEmail, customerName, productName, signLink string, deadline time.Time) error
}

type notificationService struct {
	mailer    *email.Mailer
	notifRepo repository.NotificationRepository
	log       *logger.Logger
}

func NewNotificationService(
	mailer *email.Mailer,
	notifRepo repository.NotificationRepository,
	log *logger.Logger,
) NotificationService {
	return &notificationService{
		mailer:    mailer,
		notifRepo: notifRepo,
		log:       log,
	}
}

// ── SendSubmitConfirmation ────────────────────────────────────────────────────

func (s *notificationService) SendSubmitConfirmation(
	ctx context.Context,
	appID, toEmail, customerName, productName string,
) error {
	body, err := email.RenderHTML(email.TmplSubmitConfirm, email.SubmitConfirmData{
		CustomerName: customerName,
		ProductName:  productName,
		AppID:        appID[:8],
	})
	if err != nil {
		return fmt.Errorf("render submit confirm template: %w", err)
	}

	subject := fmt.Sprintf("Pengajuan %s Anda Telah Diterima - BPR Perdana", productName)
	return s.send(ctx, appID, toEmail, "submission_confirm", subject, body)
}

// ── SendRejectionNotice ───────────────────────────────────────────────────────

func (s *notificationService) SendRejectionNotice(
	ctx context.Context,
	appID, toEmail, customerName, productName, reason string,
) error {
	body, err := email.RenderHTML(email.TmplRejection, email.RejectionData{
		CustomerName: customerName,
		ProductName:  productName,
		Reason:       reason,
	})
	if err != nil {
		return fmt.Errorf("render rejection template: %w", err)
	}

	subject := "Pemberitahuan Hasil Pengajuan - BPR Perdana"
	return s.send(ctx, appID, toEmail, "rejection_email", subject, body)
}

// ── SendApprovalNotice ────────────────────────────────────────────────────────

func (s *notificationService) SendApprovalNotice(
	ctx context.Context,
	appID, toEmail, customerName, productName string,
) error {
	body, err := email.RenderHTML(email.TmplApproval, email.ApprovalData{
		CustomerName: customerName,
		ProductName:  productName,
	})
	if err != nil {
		return fmt.Errorf("render approval template: %w", err)
	}

	subject := "Selamat! Pengajuan Anda Disetujui - BPR Perdana"
	return s.send(ctx, appID, toEmail, "approval_email", subject, body)
}

// ── private helper ────────────────────────────────────────────────────────────

func (s *notificationService) send(
	ctx context.Context,
	appID, toEmail, template, subject, body string,
) error {
	logID := uuid.New().String()
	subjectPtr := subject
	now := time.Now()

	notifLog := &model.NotificationLog{
		ID:            logID,
		ApplicationID: appID,
		Channel:       "EMAIL",
		Recipient:     toEmail,
		Template:      template,
		Subject:       &subjectPtr,
		Status:        "PENDING",
	}

	// Simpan log dulu dengan status PENDING
	if err := s.notifRepo.Create(ctx, notifLog); err != nil {
		s.log.Warn("Failed to create notification log",
			zap.String("app_id", appID),
			zap.Error(err),
		)
		// Jangan block pengiriman jika log gagal
	}

	// Kirim email
	sendErr := s.mailer.Send(email.Message{
		To:      toEmail,
		Subject: subject,
		Body:    body,
	})

	// Update log status
	if sendErr != nil {
		s.log.Error("Email send failed",
			zap.String("app_id", appID),
			zap.String("to", toEmail),
			zap.String("template", template),
			zap.Error(sendErr),
		)
		errMsg := sendErr.Error()
		notifLog.Status = "FAILED"
		notifLog.ErrorMessage = &errMsg
	} else {
		s.log.Info("Email sent",
			zap.String("app_id", appID),
			zap.String("to", toEmail),
			zap.String("template", template),
		)
		notifLog.Status = "SENT"
		notifLog.SentAt = &now
	}

	_ = s.notifRepo.Update(ctx, notifLog)

	return sendErr
}

func (s *notificationService) SendESignLink(
	ctx context.Context,
	appID, toEmail, customerName, productName, signLink string,
	deadline time.Time,
) error {
	body, err := email.RenderHTML(email.TmplESignLink, email.ESignLinkData{
		CustomerName: customerName,
		ProductName:  productName,
		SignLink:     signLink,
		Deadline:     deadline.Format("2 January 2006"),
	})
	if err != nil {
		return fmt.Errorf("render esign template: %w", err)
	}
	subject := fmt.Sprintf("Kontrak %s Siap Ditandatangani - BPR Perdana", productName)
	return s.send(ctx, appID, toEmail, "esign_link_email", subject, body)
}
