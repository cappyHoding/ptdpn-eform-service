package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cappyHoding/ptdpn-eform-service/internal/service"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// ApplicationHandler handles all customer-facing form endpoints.
type ApplicationHandler struct {
	appService service.ApplicationService
}

func NewApplicationHandler(appService service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{appService: appService}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type acceptAgreementRequest struct {
	// No body required — we capture IP and user agent from request context
}

type createApplicationRequest struct {
	AgreementToken string `json:"agreement_token" binding:"required"`
	ProductType    string `json:"product_type"    binding:"required,oneof=SAVING DEPOSIT LOAN"`

	// Product-specific (only one should be present based on product_type)
	Saving  *savingInput  `json:"saving"`
	Deposit *depositInput `json:"deposit"`
	Loan    *loanInput    `json:"loan"`
}

type savingInput struct {
	ProductName    string `json:"product_name"    binding:"required"`
	InitialDeposit uint64 `json:"initial_deposit" binding:"required,min=1"`
	SourceOfFunds  string `json:"source_of_funds" binding:"required"`
	SavingPurpose  string `json:"saving_purpose"  binding:"required"`
}

type depositInput struct {
	ProductName       string `json:"product_name"       binding:"required"`
	PlacementAmount   uint64 `json:"placement_amount"   binding:"required,min=1"`
	TenorMonths       uint8  `json:"tenor_months"       binding:"required,oneof=1 3 6 12"`
	RolloverType      string `json:"rollover_type"      binding:"required,oneof=ARO NON_ARO ARO_RATE"`
	SourceOfFunds     string `json:"source_of_funds"    binding:"required"`
	InvestmentPurpose string `json:"investment_purpose"`
}

type loanInput struct {
	ProductName     string `json:"product_name"     binding:"required"`
	RequestedAmount uint64 `json:"requested_amount" binding:"required,min=1"`
	TenorMonths     uint8  `json:"tenor_months"     binding:"required,min=1"`
	LoanPurpose     string `json:"loan_purpose"     binding:"required"`
	PaymentSource   string `json:"payment_source"   binding:"required"`
	SourceOfFunds   string `json:"source_of_funds"  binding:"required"`
}

type personalInfoRequest struct {
	// Contact info (Step 4 is where we first collect this)
	Email       string `json:"email"        binding:"required,email"`
	PhoneNumber string `json:"phone_number" binding:"required"`
	PhoneWA     string `json:"phone_wa"`

	// Additional PII not found on KTP
	MothersMaidenName string `json:"mothers_maiden_name" binding:"required"`
	Occupation        string `json:"occupation"          binding:"required"`
	WorkDuration      string `json:"work_duration"       binding:"required"`
	MonthlyIncome     uint64 `json:"monthly_income"      binding:"required,min=1"`
	Education         string `json:"education"           binding:"required"`
	WorkAddress       string `json:"work_address"`
}

// livenessRequest adalah body untuk Step 5.
// Frontend mengirim selfie base64 setelah VIDA Web SDK selesai melakukan
// liveness detection di browser. Backend yang akan call VIDA Fraud API.
type livenessRequest struct {
	// Selfie base64 dari VIDA Web SDK — tanpa data URI prefix
	// (tanpa "data:image/jpeg;base64,")
	SelfieBase64  string `json:"selfie_base64" binding:"required"`
	TransactionID string `json:"transaction_id"`
}

type disbursementRequest struct {
	BankName      string `json:"bank_name"      binding:"required"`
	BankCode      string `json:"bank_code"      binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
	AccountHolder string `json:"account_holder" binding:"required"`
}

type collateralItemRequest struct {
	CollateralType  string `json:"collateral_type"  binding:"required"`
	EstimatedValue  uint64 `json:"estimated_value"`
	OwnershipStatus string `json:"ownership_status"`
	Description     string `json:"description"`
}

type collateralRequest struct {
	Items []collateralItemRequest `json:"items" binding:"required,min=1"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// AcceptAgreement handles Step 1: POST /api/v1/applications/agree
func (h *ApplicationHandler) AcceptAgreement(c *gin.Context) {
	result, err := h.appService.AcceptAgreement(c.Request.Context(), service.AgreementInput{
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		response.InternalError(c, "")
		return
	}
	response.OK(c, "Agreement accepted. Proceed to create your application.", result)
}

// Create handles Step 2: POST /api/v1/applications
func (h *ApplicationHandler) Create(c *gin.Context) {
	var req createApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Map to service input
	input := service.CreateApplicationInput{
		AgreementToken: req.AgreementToken,
		ProductType:    req.ProductType,
		IPAddress:      c.ClientIP(),
		UserAgent:      c.Request.UserAgent(),
	}
	if req.Saving != nil {
		input.Saving = &service.SavingInput{
			ProductName:    req.Saving.ProductName,
			InitialDeposit: req.Saving.InitialDeposit,
			SourceOfFunds:  req.Saving.SourceOfFunds,
			SavingPurpose:  req.Saving.SavingPurpose,
		}
	}
	if req.Deposit != nil {
		input.Deposit = &service.DepositInput{
			ProductName:       req.Deposit.ProductName,
			PlacementAmount:   req.Deposit.PlacementAmount,
			TenorMonths:       req.Deposit.TenorMonths,
			RolloverType:      req.Deposit.RolloverType,
			SourceOfFunds:     req.Deposit.SourceOfFunds,
			InvestmentPurpose: req.Deposit.InvestmentPurpose,
		}
	}
	if req.Loan != nil {
		input.Loan = &service.LoanInput{
			ProductName:     req.Loan.ProductName,
			RequestedAmount: req.Loan.RequestedAmount,
			TenorMonths:     req.Loan.TenorMonths,
			LoanPurpose:     req.Loan.LoanPurpose,
			PaymentSource:   req.Loan.PaymentSource,
			SourceOfFunds:   req.Loan.SourceOfFunds,
		}
	}

	result, err := h.appService.CreateApplication(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMissingRequiredData):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "")
		}
		return
	}

	response.Created(c, "Application created successfully. Save your session token.", result)
}

// GetByID handles GET /api/v1/applications/:id
func (h *ApplicationHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	app, err := h.appService.GetApplicationWithDetails(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			response.NotFound(c, "Application not found")
			return
		}
		response.InternalError(c, "")
		return
	}

	response.OK(c, "Application retrieved", app)
}

// SubmitOCR handles Step 3: POST /api/v1/applications/:id/ocr
// Content-Type: application/json
//
// Frontend mengirim gambar KTP dalam bentuk base64 string (JSON body).
// Backend akan decode, kirim ke VIDA OCR API, poll hasil, lalu simpan ke DB.
//
// Request body:
//
//	{
//	  "image_base64": "/9j/4AAQ...",   ← boleh dengan/tanpa data URI prefix
//	  "filename": "ktp.jpg"            ← untuk deteksi MIME type (opsional)
//	}
func (h *ApplicationHandler) SubmitOCR(c *gin.Context) {
	appID := c.Param("id")

	var req struct {
		ImageBase64 string `json:"image_base64" binding:"required"`
		Filename    string `json:"filename"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "image_base64 is required")
		return
	}

	// Default filename jika tidak dikirim
	if req.Filename == "" {
		req.Filename = "ktp.jpg"
	}

	// ── Panggil service (decode base64 → VIDA OCR → save to DB) ──────────────
	ocrData, err := h.appService.ProcessOCR(c.Request.Context(), appID, service.OCRInput{
		ImageBase64: req.ImageBase64,
		Filename:    req.Filename,
	})
	if err != nil {
		handleAppError(c, err)
		return
	}

	response.OK(c, "KTP verified successfully. Please confirm your data.", gin.H{
		"current_step": 4,
		"nik":          ocrData.NIK,
		"full_name":    ocrData.Name,
		"birth_place":  ocrData.BirthPlace,
		"birth_date":   ocrData.BirthDate,
		"gender":       ocrData.Gender,
		"address":      ocrData.Address,
		"confidence":   ocrData.ConfidenceScore,
	})
}

func (h *ApplicationHandler) GetLivenessToken(c *gin.Context) {
	appID := c.Param("id")

	result, err := h.appService.GetLivenessToken(c.Request.Context(), appID)
	if err != nil {
		handleAppError(c, err)
		return
	}

	response.OK(c, "VIDA token retrieved", result)
}

// UpdatePersonalInfo handles Step 4: PATCH /api/v1/applications/:id/personal-info
func (h *ApplicationHandler) UpdatePersonalInfo(c *gin.Context) {
	appID := c.Param("id")

	var req personalInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid personal info: "+err.Error())
		return
	}

	err := h.appService.UpdatePersonalInfo(c.Request.Context(), appID, service.PersonalInfoInput{
		Email:             req.Email,
		PhoneNumber:       req.PhoneNumber,
		PhoneWA:           req.PhoneWA,
		MothersMaidenName: req.MothersMaidenName,
		Occupation:        req.Occupation,
		WorkDuration:      req.WorkDuration,
		MonthlyIncome:     req.MonthlyIncome,
		Education:         req.Education,
		WorkAddress:       req.WorkAddress,
	})
	if err != nil {
		handleAppError(c, err)
		return
	}

	response.OK(c, "Personal information saved. Proceed to Step 5.", gin.H{"current_step": 5})
}

// SubmitLiveness handles Step 5: POST /api/v1/applications/:id/liveness
//
// FLOW:
//  1. Customer menyelesaikan VIDA Web SDK di browser (liveness detection)
//  2. Frontend mengambil selfie base64 dari hasil Web SDK
//  3. Frontend kirim ke endpoint ini
//  4. Backend call VIDA Fraud Mitigation API:
//     - Ambil NIK + nama + DOB dari hasil OCR Step 3
//     - Kirim selfie untuk face match + verifikasi ke Dukcapil
//     - transactionType: FULL_FRAUD_ASSESSMENT
//  5. Simpan hasil dan advance ke Step 6
func (h *ApplicationHandler) SubmitLiveness(c *gin.Context) {
	appID := c.Param("id")

	var req livenessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "selfie_base64 is required")
		return
	}

	err := h.appService.SaveLivenessResult(c.Request.Context(), appID, service.LivenessInput{
		SelfieBase64: req.SelfieBase64,
	})
	if err != nil {
		handleAppError(c, err)
		return
	}

	response.OK(c, "Identity verification completed. Proceed to Step 6.", gin.H{"current_step": 6})
}

// UpdateDisbursement handles Step 6: PATCH /api/v1/applications/:id/disbursement
func (h *ApplicationHandler) UpdateDisbursement(c *gin.Context) {
	appID := c.Param("id")

	var req disbursementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid disbursement data: "+err.Error())
		return
	}

	err := h.appService.UpdateDisbursement(c.Request.Context(), appID, service.DisbursementInput{
		BankName:      req.BankName,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		AccountHolder: req.AccountHolder,
	})
	if err != nil {
		handleAppError(c, err)
		return
	}

	response.OK(c, "Bank account saved. Proceed to Step 7 for review.", gin.H{"current_step": 7})
}

// Submit handles Step 7: POST /api/v1/applications/:id/submit
func (h *ApplicationHandler) Submit(c *gin.Context) {
	appID := c.Param("id")

	if err := h.appService.Submit(c.Request.Context(), appID); err != nil {
		handleAppError(c, err)
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Success: true,
		Message: "Application submitted successfully! Our team will review it shortly.",
		Data: gin.H{
			"application_id": appID,
			"status":         "PENDING_REVIEW",
		},
	})
}

func (h *ApplicationHandler) SubmitCollateral(c *gin.Context) {
	appID := c.Param("id")

	var req collateralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid collateral data: %s", err.Error()),
		})
		return
	}

	items := make([]service.CollateralItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.CollateralItemInput{
			CollateralType:  item.CollateralType,
			EstimatedValue:  item.EstimatedValue,
			OwnershipStatus: item.OwnershipStatus,
			Description:     item.Description,
		})
	}

	if err := h.appService.SaveCollateral(c.Request.Context(), appID, service.CollateralInput{
		Items: items,
	}); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrApplicationNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, service.ErrStepNotComplete) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Collateral data saved"})
}

// TrackStatus handles GET /api/v1/applications/track?id=APP_ID
// Public endpoint — tidak perlu session token.
// Nasabah cek status pengajuan menggunakan application ID dari konfirmasi submit.
func (h *ApplicationHandler) TrackStatus(c *gin.Context) {
	appID := c.Query("id")
	if appID == "" {
		response.BadRequest(c, "Application ID is required")
		return
	}

	// searchID := appID
	// prefixes := []string{"DEP-", "TAB-", "KRD-"}
	// for _, p := range prefixes {
	// 	if strings.HasPrefix(strings.ToUpper(appID), p) {
	// 		searchID = appID[len(p):]
	// 		break
	// 	}
	// }

	app, err := h.appService.GetApplicationWithDetails(c.Request.Context(), appID)
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			response.NotFound(c, "Pengajuan tidak ditemukan. Pastikan ID pengajuan sudah benar.")
			return
		}
		response.InternalError(c, "")
		return
	}

	statusMessages := map[string]string{
		"DRAFT":          "Pengajuan Anda belum selesai diisi.",
		"PENDING_REVIEW": "Pengajuan Anda telah diterima dan sedang menunggu review oleh tim kami.",
		"IN_REVIEW":      "Pengajuan Anda sedang dalam proses review oleh tim kami.",
		"RECOMMENDED":    "Pengajuan Anda sedang dalam proses persetujuan akhir.",
		"APPROVED":       "Selamat! Pengajuan Anda telah disetujui. Silakan cek email Anda.",
		"REJECTED":       "Mohon maaf, pengajuan Anda tidak dapat diproses. Hubungi CS kami.",
		"SIGNING":        "Silakan tandatangani kontrak melalui link yang telah dikirim ke email Anda.",
		"COMPLETED":      "Proses pengajuan Anda telah selesai. Selamat bergabung dengan BPR Perdana!",
		"EXPIRED":        "Masa berlaku pengajuan Anda telah habis. Silakan hubungi CS kami.",
	}

	statusLabels := map[string]string{
		"DRAFT": "Draft", "PENDING_REVIEW": "Menunggu Review",
		"IN_REVIEW": "Dalam Review", "RECOMMENDED": "Direkomendasikan",
		"APPROVED": "Disetujui", "REJECTED": "Ditolak",
		"SIGNING": "Menunggu Tanda Tangan", "COMPLETED": "Selesai",
		"EXPIRED": "Kedaluwarsa",
	}

	// ── Customer name — Customer adalah value type, cek via pointer field ──────
	customerName := ""
	if app.Customer.FullName != nil { // ← langsung akses field, BUKAN cek app.Customer != nil
		name := *app.Customer.FullName
		if len(name) > 3 {
			customerName = name[:3] + strings.Repeat("*", len(name)-3)
		} else {
			customerName = name
		}
	}

	var submittedAt *string
	if app.SubmittedAt != nil {
		t := app.SubmittedAt.Format("02 Jan 2006 15:04")
		submittedAt = &t
	}

	type statusResponse struct {
		ApplicationID string  `json:"application_id"`
		Status        string  `json:"status"`
		StatusLabel   string  `json:"status_label"`
		ProductType   string  `json:"product_type"`
		CustomerName  string  `json:"customer_name,omitempty"`
		SubmittedAt   *string `json:"submitted_at,omitempty"`
		UpdatedAt     string  `json:"updated_at"`
		Message       string  `json:"message"`
	}

	response.OK(c, "Status retrieved", statusResponse{
		ApplicationID: app.ID,
		Status:        string(app.Status),
		StatusLabel:   statusLabels[string(app.Status)],
		ProductType:   string(app.ProductType),
		CustomerName:  customerName,
		SubmittedAt:   submittedAt,
		UpdatedAt:     app.UpdatedAt.Format("02 Jan 2006 15:04"),
		Message:       statusMessages[string(app.Status)],
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// handleAppError maps service errors to HTTP responses.
func handleAppError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApplicationNotFound):
		response.NotFound(c, "Application not found")
	case errors.Is(err, service.ErrAlreadySubmitted):
		response.UnprocessableEntity(c, "This application has already been submitted")
	case errors.Is(err, service.ErrStepNotComplete):
		response.UnprocessableEntity(c, err.Error())
	case errors.Is(err, service.ErrMissingRequiredData):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrDisbursementRequired):
		response.BadRequest(c, "Bank account information is required for this product type")
	default:
		response.InternalError(c, "")
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
