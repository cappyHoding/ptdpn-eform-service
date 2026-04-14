package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/config"
	"github.com/cappyHoding/ptdpn-eform-service/internal/integration/vida"
	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	contractpdf "github.com/cappyHoding/ptdpn-eform-service/pkg/pdf"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ContractService orchestrates the full contract lifecycle after approval:
//
//	APPROVE → GeneratePDF → eMeterai → eSign → status: SIGNING
//	Webhook  → SaveSignedPDF → status: COMPLETED
type ContractService interface {
	// InitiateContract dipanggil oleh ApproveApplication setelah supervisor approve.
	InitiateContract(ctx context.Context, appID string, actorID string) error

	// CompleteContract dipanggil oleh webhook VIDA ketika customer selesai TTD.
	CompleteContract(ctx context.Context, vidaTransactionID string, signedPDFBase64 string) error

	// FailContract dipanggil webhook ketika sign expired/failed.
	FailContract(ctx context.Context, vidaTransactionID string, reason string) error
}

type contractService struct {
	appRepo      repository.ApplicationRepository
	contractRepo repository.ContractRepository
	auditRepo    repository.AuditRepository
	vida         *vida.Services
	storage      *storage.Manager
	logoPath     string              // path ke file logo perusahaan
	serverIP     string              // IP server untuk eSign requestInfo.srcIp
	notifSvc     NotificationService // ← tambah ini
	cfg          *config.Config
	log          *logger.Logger
}

func NewContractService(
	appRepo repository.ApplicationRepository,
	contractRepo repository.ContractRepository,
	auditRepo repository.AuditRepository,
	vidaServices *vida.Services,
	storageManager *storage.Manager,
	logoPath string,
	notifSvc NotificationService, // ← tambah ini
	cfg *config.Config, // ← tambah ini
	log *logger.Logger,
) ContractService {
	return &contractService{
		appRepo:      appRepo,
		contractRepo: contractRepo,
		auditRepo:    auditRepo,
		vida:         vidaServices,
		storage:      storageManager,
		logoPath:     logoPath,
		serverIP:     getOutboundIP(),
		notifSvc:     notifSvc, // ← set ini
		cfg:          cfg,      // ← set ini
		log:          log,
	}
}

// getOutboundIP mendapatkan IP outbound server.
// Digunakan sebagai srcIp pada request eSign ke VIDA.
func getOutboundIP() string {
	// Dial UDP ke public DNS — tidak benar-benar konek, hanya untuk resolve IP lokal
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// ─── InitiateContract ─────────────────────────────────────────────────────────

func (s *contractService) InitiateContract(ctx context.Context, appID string, actorID string) error {
	// 1. Load application dengan semua data
	app, err := s.appRepo.FindByIDWithDetails(ctx, appID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return ErrApplicationNotFound
		}
		return fmt.Errorf("load application failed: %w", err)
	}

	// 2. Dapatkan email customer
	customerEmail := ""
	if app.Customer.Email != nil {
		customerEmail = *app.Customer.Email
	}
	if customerEmail == "" {
		return fmt.Errorf("customer email not found — cannot register eSign")
	}

	// ── Cek mock mode dulu sebelum panggil VIDA apapun ─────────────────────
	if s.cfg.Vida.MockContract {
		return s.initiateMockContract(ctx, app, appID, actorID, customerEmail)
	}

	// 3. Generate PDF kontrak
	s.log.Info("Generating contract PDF", zap.String("app_id", appID))
	pdfBytes, err := contractpdf.GenerateContract(buildContractData(app, s.logoPath))
	if err != nil {
		return fmt.Errorf("PDF generation failed: %w", err)
	}

	pdfPath, err := s.storage.SaveFile(
		storage.FileTypeContract,
		appID+".pdf",
		bytes.NewReader(pdfBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to save contract PDF: %w", err)
	}
	s.log.Info("Contract PDF saved", zap.String("path", pdfPath))

	// 4. Apply eMeterai (best-effort — tidak block jika gagal)
	var materaiID *string
	var materaiAppliedAt *time.Time

	stampCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	stampResult, stampErr := s.vida.EMeterai.ApplyStamp(stampCtx, pdfPath, appID)
	if stampErr != nil {
		s.log.Warn("eMeterai failed, proceeding without stamp",
			zap.String("app_id", appID), zap.Error(stampErr))
	} else if stampResult != nil && stampResult.StampedDoc != "" {
		stampedBytes, decErr := base64.StdEncoding.DecodeString(stampResult.StampedDoc)
		if decErr == nil {
			if stamppedPath, saveErr := s.storage.SaveFile(
				storage.FileTypeContract, appID+".pdf",
				bytes.NewReader(stampedBytes),
			); saveErr == nil {
				pdfPath = stamppedPath
				now := time.Now()
				materaiAppliedAt = &now
				mid := stampResult.MateraiID
				materaiID = &mid
				s.log.Info("eMeterai applied", zap.String("serial", stampResult.SerialNumber))
			}
		}
	}

	// 5. Register eSign
	signTrxID, err := s.vida.Sign.RegisterDocument(
		ctx,
		pdfPath,
		customerEmail,
		fmt.Sprintf("Kontrak_%s.pdf", appID[:8]),
		s.serverIP,
		"BPR-Perdana-Backend/1.0",
	)
	if err != nil {
		return fmt.Errorf("eSign registration failed: %w", err)
	}
	s.log.Info("eSign registered",
		zap.String("app_id", appID),
		zap.String("sign_trx_id", signTrxID),
	)

	// 6. Simpan contract document ke DB
	fileSize := uint32(len(pdfBytes))
	now := time.Now()
	deadline := now.Add(7 * 24 * time.Hour)

	doc := &model.ContractDocument{
		ID:                uuid.New().String(),
		ApplicationID:     appID,
		DocumentType:      string(app.ProductType),
		FilePath:          pdfPath,
		FileSizeBytes:     &fileSize,
		VidaSignRequestID: &signTrxID,
		EMateraiID:        materaiID,
		EMateraiAppliedAt: materaiAppliedAt,
		SignStatus:        "PENDING",
		SignDeadline:      &deadline,
		GeneratedAt:       now,
	}
	if err := s.contractRepo.CreateContract(ctx, doc); err != nil {
		return fmt.Errorf("failed to save contract record: %w", err)
	}

	// 7. Update status → SIGNING
	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusSigning); err != nil {
		return fmt.Errorf("failed to update status to SIGNING: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "internal_user",
		ActorID:     &actorID,
		Action:      "CONTRACT_INITIATED",
		EntityType:  strPtrIfNotEmpty("application"),
		EntityID:    strPtrIfNotEmpty(appID),
		Description: strPtrIfNotEmpty("Contract PDF generated, eMeterai applied, eSign link sent to " + customerEmail),
		NewValue:    model.JSON{"status": "SIGNING", "sign_trx_id": signTrxID},
	})

	return nil
}

func (s *contractService) initiateMockContract(
	ctx context.Context,
	app *model.Application,
	appID, actorID, customerEmail string,
) error {
	now := time.Now()
	deadline := now.Add(7 * 24 * time.Hour)
	mockSignLink := fmt.Sprintf("https://sign.vida.id/mock/%s", appID[:8])

	doc := &model.ContractDocument{
		ID:             uuid.New().String(),
		ApplicationID:  appID,
		DocumentType:   string(app.ProductType),
		FilePath:       "mock/contract/" + appID + ".pdf",
		SignStatus:     "SIGNING",
		SignLink:       &mockSignLink,
		SignLinkSentAt: &now,
		SignDeadline:   &deadline,
		GeneratedAt:    now,
	}
	if err := s.contractRepo.CreateContract(ctx, doc); err != nil {
		return fmt.Errorf("failed to save mock contract: %w", err)
	}

	if err := s.appRepo.UpdateStatus(ctx, appID, model.StatusSigning); err != nil {
		return fmt.Errorf("failed to update status to SIGNING: %w", err)
	}

	if s.notifSvc != nil {
		customerName := "Nasabah"
		if app.Customer.FullName != nil {
			customerName = *app.Customer.FullName
		}
		_ = s.notifSvc.SendESignLink(
			context.Background(),
			appID, customerEmail, customerName,
			string(app.ProductType),
			mockSignLink, deadline,
		)
	}

	s.log.Info("Mock contract initiated",
		zap.String("app_id", appID),
		zap.String("sign_link", mockSignLink),
	)

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "system",
		Action:      "CONTRACT_INITIATED",
		EntityType:  strPtrIfNotEmpty("application"),
		EntityID:    strPtrIfNotEmpty(appID),
		Description: strPtrIfNotEmpty("Mock contract, eSign link sent to " + customerEmail),
		NewValue:    model.JSON{"status": "SIGNING", "mock": true},
	})

	return nil
}

// ─── CompleteContract ─────────────────────────────────────────────────────────

func (s *contractService) CompleteContract(ctx context.Context, vidaTransactionID string, signedPDFBase64 string) error {
	doc, err := s.contractRepo.FindContractBySignTrxID(ctx, vidaTransactionID)
	if err != nil {
		return fmt.Errorf("contract not found for trx %s: %w", vidaTransactionID, err)
	}

	// Simpan signed PDF jika ada dalam payload webhook
	if signedPDFBase64 != "" {
		if signedBytes, decErr := base64.StdEncoding.DecodeString(signedPDFBase64); decErr == nil {
			signedPath, saveErr := s.storage.SaveFile(
				storage.FileTypeContract,
				doc.ApplicationID+"_signed.pdf",
				bytes.NewReader(signedBytes),
			)
			if saveErr == nil {
				doc.SignedFilePath = &signedPath
			}
		}
	}

	now := time.Now()
	doc.SignStatus = "COMPLETED"
	doc.SignedAt = &now

	if err := s.contractRepo.UpdateContract(ctx, doc); err != nil {
		return fmt.Errorf("failed to update contract: %w", err)
	}
	if err := s.appRepo.UpdateStatus(ctx, doc.ApplicationID, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update application status: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "customer",
		Action:      "CONTRACT_SIGNED",
		EntityType:  strPtrIfNotEmpty("application"),
		EntityID:    strPtrIfNotEmpty(doc.ApplicationID),
		Description: strPtrIfNotEmpty("Customer signed contract via VIDA eSign"),
		NewValue:    model.JSON{"status": "COMPLETED", "signed_at": now.Format(time.RFC3339)},
	})

	s.log.Info("Contract COMPLETED",
		zap.String("app_id", doc.ApplicationID),
		zap.String("sign_trx_id", vidaTransactionID),
	)
	return nil
}

// ─── FailContract ─────────────────────────────────────────────────────────────

func (s *contractService) FailContract(ctx context.Context, vidaTransactionID string, reason string) error {
	doc, err := s.contractRepo.FindContractBySignTrxID(ctx, vidaTransactionID)
	if err != nil {
		return fmt.Errorf("contract not found for trx %s: %w", vidaTransactionID, err)
	}

	doc.SignStatus = "EXPIRED"
	if err := s.contractRepo.UpdateContract(ctx, doc); err != nil {
		return fmt.Errorf("failed to update contract: %w", err)
	}
	// Rollback ke APPROVED — operator bisa trigger ulang
	if err := s.appRepo.UpdateStatus(ctx, doc.ApplicationID, model.StatusApproved); err != nil {
		return fmt.Errorf("failed to revert status: %w", err)
	}

	s.writeAudit(ctx, &model.AuditLog{
		ActorType:   "system",
		Action:      "CONTRACT_SIGN_FAILED",
		EntityType:  strPtrIfNotEmpty("application"),
		EntityID:    strPtrIfNotEmpty(doc.ApplicationID),
		Description: strPtrIfNotEmpty("eSign expired/failed: " + reason),
		NewValue:    model.JSON{"status": "APPROVED", "reason": reason},
	})
	return nil
}

// ─── Private helpers ──────────────────────────────────────────────────────────

func (s *contractService) writeAudit(ctx context.Context, entry *model.AuditLog) {
	entry.CreatedAt = time.Now()
	if err := s.auditRepo.Write(ctx, entry); err != nil {
		s.log.Error("audit write failed", zap.String("action", entry.Action), zap.Error(err))
	}
}

// buildContractData maps model.Application → contractpdf.ContractData
func buildContractData(app *model.Application, logoPath string) contractpdf.ContractData {
	data := contractpdf.ContractData{
		ApplicationID: app.ID,
		ProductType:   string(app.ProductType),
		ApprovedAt:    time.Now(),
		LogoPath:      logoPath,
	}

	// Data dari OCR KTP
	if app.OCRResult != nil {
		data.FullName = derefStr(app.OCRResult.FullName)
		data.NIK = derefStr(app.OCRResult.NIK)
		data.BirthPlace = derefStr(app.OCRResult.BirthPlace)
		data.BirthDate = derefStr(app.OCRResult.BirthDate)
		data.Address = derefStr(app.OCRResult.Address)
		data.Kelurahan = derefStr(app.OCRResult.Kelurahan)
		data.Kecamatan = derefStr(app.OCRResult.Kecamatan)
		data.KabupatenKota = derefStr(app.OCRResult.KabupatenKota)
		data.Provinsi = derefStr(app.OCRResult.Provinsi)
		data.Occupation = derefStr(app.OCRResult.Occupation)
		data.Nationality = derefStr(app.OCRResult.Nationality)
	}

	// Data dari Customer (personal info diisi saat Step 4)
	data.Email = derefStr(app.Customer.Email)
	data.PhoneNumber = derefStr(app.Customer.PhoneNumber)
	data.Education = derefStr(app.Customer.Education)
	data.MonthlyIncome = derefUint64(app.Customer.MonthlyIncome)
	data.MothersMaidenName = derefStr(app.Customer.MothersMaidenName)

	// Detail produk
	switch app.ProductType {
	case model.ProductSaving:
		if app.SavingDetail != nil {
			data.Saving = &contractpdf.SavingData{
				ProductName:    app.SavingDetail.ProductName,
				InitialDeposit: app.SavingDetail.InitialDeposit,
				SourceOfFunds:  app.SavingDetail.SourceOfFunds,
				SavingPurpose:  app.SavingDetail.SavingPurpose,
			}
		}
	case model.ProductDeposit:
		if app.DepositDetail != nil {
			data.Deposit = &contractpdf.DepositData{
				ProductName:       app.DepositDetail.ProductName,
				PlacementAmount:   app.DepositDetail.PlacementAmount,
				TenorMonths:       app.DepositDetail.TenorMonths,
				RolloverType:      app.DepositDetail.RolloverType,
				SourceOfFunds:     app.DepositDetail.SourceOfFunds,
				InvestmentPurpose: derefStr(app.DepositDetail.InvestmentPurpose),
			}
		}
	case model.ProductLoan:
		if app.LoanDetail != nil {
			data.Loan = &contractpdf.LoanData{
				ProductName:     app.LoanDetail.ProductName,
				RequestedAmount: app.LoanDetail.RequestedAmount,
				TenorMonths:     app.LoanDetail.TenorMonths,
				LoanPurpose:     app.LoanDetail.LoanPurpose,
				PaymentSource:   app.LoanDetail.PaymentSource,
				SourceOfFunds:   app.LoanDetail.SourceOfFunds,
			}
		}
	}

	// Rekening pencairan
	if app.DisbursementData != nil {
		data.BankName = app.DisbursementData.BankName
		data.BankCode = app.DisbursementData.BankCode
		data.AccountNumber = app.DisbursementData.AccountNumber
		data.AccountHolder = app.DisbursementData.AccountHolder
	}

	return data
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Now()
	}
	return *t
}
func derefUint64(u *uint64) uint64 {
	if u == nil {
		return 0
	}
	return *u
}
