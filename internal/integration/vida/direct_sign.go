package vida

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DirectSignService menangani VIDA Direct Sign Platform API.
//
// ALUR:
//
//  1. CreateEnvelope → POST /core/external-api/rest/v1/envelope (multipart/form-data)
//     Input: PDF file + envelope_details JSON + creator_email
//     Output: envelope_id
//
//  2. StartSignature → POST /core/external-api/rest/v1/envelope/{id}/start-signature
//     Output: signature_links[].signature_link — URL yang dikirim ke nasabah
//
//  3. Nasabah buka URL → auth OTP/biometric di VIDA → TTD
//
//  4. VIDA kirim Signature Event Webhook → backend update status COMPLETED
//
// BERBEDA DENGAN PoA Sign:
//   - PoA: backend TTD atas nama nasabah (tidak perlu interaksi nasabah)
//   - Direct Sign: nasabah TTD sendiri menggunakan sertifikat elektroniknya di VIDA
//   - Direct Sign butuh kyc_event_id dari hasil liveness verification nasabah
type DirectSignService struct {
	client       *Client
	creatorEmail string // email member workspace VIDA (VIDA_DSIGN_CREATOR_EMAIL)
	log          *zap.Logger

	dsignToken       string
	dsignTokenExpiry time.Time
	dsignMu          sync.Mutex
}

// NewDirectSignService membuat DirectSignService baru.
func NewDirectSignService(client *Client, creatorEmail string) *DirectSignService {
	logger, _ := zap.NewProduction()
	return &DirectSignService{
		client:       client,
		creatorEmail: creatorEmail,
		log:          logger,
	}
}

// ─── Request types ────────────────────────────────────────────────────────────

// directSignRecipient adalah penerima dokumen untuk Direct Sign.
type directSignRecipient struct {
	Email             string         `json:"email"`
	Name              string         `json:"name"`
	KYCEventID        string         `json:"kyc_event_id,omitempty"` // dari liveness_results.vida_request_id
	SignatureSpecimen *dsignSpecimen `json:"signature_specimen,omitempty"`
}

type dsignSpecimen struct {
	Type string `json:"type"` // "STANDARD" | "IMAGE_UPLOAD" | "QR_CODE"
}

// directSignField mendefinisikan posisi kotak tanda tangan di halaman PDF.
type directSignField struct {
	Type           string `json:"type"`            // "Signature" | "QR"
	X              int    `json:"x"`               // koordinat pojok kiri atas
	Y              int    `json:"y"`               // y=0 dari atas
	Width          int    `json:"width"`           // lebar kotak TTD
	Height         int    `json:"height"`          // tinggi kotak TTD
	PageIndex      int    `json:"page_index"`      // 1-based
	RecipientEmail string `json:"recipient_email"` // harus sama dengan recipients[].email
}

// directSignEnvelopeDetails adalah JSON string yang di-pass ke field envelope_details.
type directSignEnvelopeDetails struct {
	EnvelopeName   string                `json:"envelope_name"`
	DirectSign     bool                  `json:"direct_sign"`    // WAJIB true
	Preview        bool                  `json:"preview"`        // nasabah bisa preview sebelum TTD
	NotifyEmail    bool                  `json:"notify_email"`   // false = kita handle email sendiri
	SignatureType  string                `json:"signature_type"` // "DIGITAL" = legal binding
	ExpirationDays int                   `json:"expiration_days"`
	Recipients     []directSignRecipient `json:"recipients"`
	Fields         []directSignField     `json:"fields"`
}

// ─── Response types ───────────────────────────────────────────────────────────

type dsignCreateEnvelopeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID string `json:"id"` // envelope_id
	} `json:"data"`
}

type dsignSignatureLink struct {
	Email         string `json:"email"`
	SignatureLink string `json:"signature_link"` // URL ke VIDA Sign Platform
}

type dsignStartSignatureResponse struct {
	Success bool `json:"success"`
	Data    struct {
		EnvelopeID     string               `json:"envelope_id"`
		EnvelopeStatus string               `json:"envelope_status"` // "PENDING"
		SignatureLinks []dsignSignatureLink `json:"signature_links"`
	} `json:"data"`
}

// SignatureEventWebhook adalah payload yang VIDA kirim ke webhook kita.
// Didaftarkan di VIDA dashboard / TAM.
type SignatureEventWebhook struct {
	EventType  string `json:"event_type"` // "ENVELOPE_SIGNED" | "ENVELOPE_EXPIRED" | "ENVELOPE_DECLINED"
	EnvelopeID string `json:"envelope_id"`
	Status     string `json:"status"`
	SignedAt   string `json:"signed_at,omitempty"`
}

// ─── Input/Output structs ─────────────────────────────────────────────────────

// CreateEnvelopeInput adalah parameter untuk CreateAndStartEnvelope.
type CreateEnvelopeInput struct {
	PDFPath        string // path ke file PDF di storage lokal
	EnvelopeName   string
	RecipientName  string
	RecipientEmail string
	KYCEventID     string // vida_request_id dari liveness_results — wajib untuk Direct Sign

	ExpirationDays int // default 7

	// Koordinat kotak tanda tangan di PDF
	// x, y = pojok kiri atas (x=0 dari kiri, y=0 dari atas)
	// width, height = ukuran kotak
	// PageIndex = halaman (1-based)
	SignX         int
	SignY         int
	SignWidth     int
	SignHeight    int
	SignPageIndex int
}

// CreateEnvelopeResult adalah output dari CreateAndStartEnvelope.
type CreateEnvelopeResult struct {
	EnvelopeID    string // simpan ke contract_documents.vida_sign_request_id
	SignatureLink string // URL yang dikirim ke nasabah via email + SMS
}

// ─── Public methods ───────────────────────────────────────────────────────────

// CreateAndStartEnvelope melakukan Create Envelope + Start Signature dalam satu panggilan.
// Mengembalikan envelope_id dan signing_link yang siap dikirim ke nasabah.
func (s *DirectSignService) CreateAndStartEnvelope(
	ctx context.Context,
	input CreateEnvelopeInput,
) (*CreateEnvelopeResult, error) {
	// Validasi file ada
	if _, err := os.Stat(input.PDFPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file not found: %s", input.PDFPath)
	}

	// Default values
	if input.ExpirationDays <= 0 {
		input.ExpirationDays = 7
	}
	if input.SignWidth == 0 {
		input.SignWidth = 200
	}
	if input.SignHeight == 0 {
		input.SignHeight = 60
	}
	if input.SignPageIndex == 0 {
		input.SignPageIndex = 1
	}

	// Step 1: Create Envelope
	envelopeID, err := s.createEnvelope(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create envelope: %w", err)
	}
	s.log.Info("Direct Sign envelope created",
		zap.String("envelope_id", envelopeID),
		zap.String("recipient", input.RecipientEmail),
	)

	// Step 2: Start Signature → dapat signing URL
	signingLink, err := s.startSignature(ctx, envelopeID, input.RecipientEmail)
	if err != nil {
		return nil, fmt.Errorf("start signature: %w", err)
	}
	s.log.Info("Direct Sign signature started",
		zap.String("envelope_id", envelopeID),
		zap.String("signing_url", signingLink),
	)

	return &CreateEnvelopeResult{
		EnvelopeID:    envelopeID,
		SignatureLink: signingLink,
	}, nil
}

// ─── Private methods ──────────────────────────────────────────────────────────

func (s *DirectSignService) createEnvelope(ctx context.Context, input CreateEnvelopeInput) (string, error) {
	// Buka file PDF
	pdfFile, err := os.Open(input.PDFPath)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer pdfFile.Close()

	if info, statErr := pdfFile.Stat(); statErr == nil {
		s.log.Info("Uploading PDF to VIDA",
			zap.String("path", input.PDFPath),
			zap.Int64("size_bytes", info.Size()),
		)
		if info.Size() == 0 {
			return "", fmt.Errorf("PDF file is empty: %s", input.PDFPath)
		}
	}

	// signatureType := "DIGITAL" // legal binding, PSrE VIDA
	// if input.KYCEventID == "" {
	// 	signatureType = "ESIGN"
	// }

	// Build envelope_details JSON
	details := directSignEnvelopeDetails{
		EnvelopeName:   input.EnvelopeName,
		DirectSign:     true,    // WAJIB true
		Preview:        true,    // nasabah bisa preview dokumen sebelum TTD
		NotifyEmail:    false,   // kita handle email sendiri via notification_service
		SignatureType:  "ESIGN", // "DIGITAL" = legal binding, "ESIGN" = non-binding
		ExpirationDays: input.ExpirationDays,

		Recipients: []directSignRecipient{
			{
				Email:      input.RecipientEmail,
				Name:       input.RecipientName,
				KYCEventID: input.KYCEventID,
				SignatureSpecimen: &dsignSpecimen{
					Type: "STANDARD", // nama + VIDA certification mark
				},
			},
		},
		Fields: []directSignField{
			{
				Type:           "Signature",
				X:              input.SignX,
				Y:              input.SignY,
				Width:          input.SignWidth,
				Height:         input.SignHeight,
				PageIndex:      input.SignPageIndex,
				RecipientEmail: input.RecipientEmail,
			},
		},
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return "", fmt.Errorf("marshal envelope_details: %w", err)
	}

	// Build multipart/form-data
	// TIDAK menggunakan client.doJSON karena Content-Type harus multipart/form-data
	// Kita ambil token dari client secara manual
	token, err := s.getDSToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}

	s.log.Info("Direct Sign envelope_details",
		zap.String("json", string(detailsJSON)),
		zap.String("creator_email", s.creatorEmail),
		zap.String("pdf_path", input.PDFPath),
	)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Field: creator_email
	if err := writer.WriteField("creator_email", s.creatorEmail); err != nil {
		return "", fmt.Errorf("write creator_email: %w", err)
	}

	// Field: envelope_details (JSON string)
	if err := writer.WriteField("envelope_details", string(detailsJSON)); err != nil {
		return "", fmt.Errorf("write envelope_details: %w", err)
	}

	// Field: file (PDF binary)
	// h := make(textproto.MIMEHeader)
	// h.Set("Content-Disposition", `form-data; name="file"; filename="kontrak.pdf"`)
	// h.Set("Content-Type", "application/pdf")
	// part, err := writer.CreatePart(h)
	part, err := writer.CreateFormFile("file", "kontrak.pdf")
	if err != nil {
		return "", fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(part, pdfFile); err != nil {
		return "", fmt.Errorf("copy PDF to multipart: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	// HTTP request — manual karena multipart
	url := s.client.baseURL + "/core/external-api/rest/v1/envelope"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP POST envelope: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VIDA create envelope HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result dsignCreateEnvelopeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal create envelope: %w — body: %s", err, string(respBody))
	}
	if !result.Success || result.Data.ID == "" {
		return "", fmt.Errorf("VIDA create envelope no ID returned: %s", string(respBody))
	}

	return result.Data.ID, nil
}

func (s *DirectSignService) startSignature(ctx context.Context, envelopeID, recipientEmail string) (string, error) {
	token, err := s.getDSToken(ctx) // ← GANTI INI
	if err != nil {
		return "", fmt.Errorf("get Direct Sign token: %w", err)
	}

	url := fmt.Sprintf("%s/core/external-api/rest/v1/envelope/%s/start-signature",
		s.client.baseURL, envelopeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP POST start-signature: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("start-signature HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result dsignStartSignatureResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal start-signature: %w", err)
	}
	if !result.Success || len(result.Data.SignatureLinks) == 0 {
		return "", fmt.Errorf("no signature links returned: %s", string(respBody))
	}

	for _, sl := range result.Data.SignatureLinks {
		if sl.Email == recipientEmail {
			return sl.SignatureLink, nil
		}
	}
	return result.Data.SignatureLinks[0].SignatureLink, nil
}

// getDSToken mendapatkan access token dari Direct Sign OAuth endpoint.
// Endpoint: POST {baseURL}/core/api/rest/v1/oauth2/token
// BERBEDA dari OCR/Fraud yang pakai SSO terpisah.
func (s *DirectSignService) getDSToken(ctx context.Context) (string, error) {
	s.dsignMu.Lock()
	defer s.dsignMu.Unlock()

	// Pakai cached token jika masih valid (buffer 60 detik)
	if s.dsignToken != "" && time.Now().Add(60*time.Second).Before(s.dsignTokenExpiry) {
		return s.dsignToken, nil
	}

	tokenURL := s.client.baseURL + "/core/api/rest/v1/oauth2/token"

	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", s.client.clientID)
	formData.Set("client_secret", s.client.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Direct Sign token HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	s.dsignToken = tokenResp.AccessToken
	s.dsignTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	s.log.Info("Direct Sign token refreshed",
		zap.Int("expires_in", tokenResp.ExpiresIn))

	return s.dsignToken, nil
}
