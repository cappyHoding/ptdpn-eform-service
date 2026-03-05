package vida

import "time"

// ─── Authentication ───────────────────────────────────────────────────────────

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// ─── OCR KTP ─────────────────────────────────────────────────────────────────
// Endpoint: POST /verify/v1/ktp/ocr/transaction
// Body: JSON dengan ktp_image sebagai data URI base64

type OCRRequest struct {
	Parameters OCRParameters `json:"parameters"`
}

type OCRParameters struct {
	// Format: "data:image/jpeg;base64,/9j/..."
	KTPImage string `json:"ktp_image"`
}

// OCRResponse adalah response dari POST /verify/v1/ktp/ocr/transaction
// VIDA langsung mengembalikan hasil lengkap di response submit — tidak perlu polling.
//
// Struktur aktual dari VIDA sandbox:
//   {
//     "data": {
//       "result": { "nik": "...", "nama": "...", ... },
//       "status": "success",
//       "transaction_id": "uuid"
//     }
//   }
type OCRResponse struct {
	Data OCRResponseData `json:"data"`
}

type OCRResponseData struct {
	Result        OCRData `json:"result"`
	Status        string  `json:"status"` // "success" | "failed"
	TransactionID string  `json:"transaction_id"`
}

// OCRData berisi semua field yang diekstrak dari KTP.
// Nama field menggunakan bahasa Indonesia sesuai response aktual VIDA.
type OCRData struct {
	TransactionID string `json:"-"` // diisi manual dari OCRResponseData.TransactionID

	// Field sesuai response aktual VIDA
	NIK             string  `json:"nik"`
	Name            string  `json:"nama"`          // "nama" bukan "name"
	BirthPlace      string  `json:"tempat_lahir"`  // "tempat_lahir" bukan "birth_place"
	BirthDate       string  `json:"tanggal_lahir"` // "tanggal_lahir" bukan "birth_date", format DD-MM-YYYY
	Gender          string  `json:"jenis_kelamin"` // "jenis_kelamin" bukan "gender"
	Address         string  `json:"alamat"`        // "alamat" bukan "address"
	RTRW            string  `json:"rt_rw"`
	Kelurahan       string  `json:"kelurahan_desa"` // "kelurahan_desa" bukan "kel_desa"
	Kecamatan       string  `json:"kecamatan"`
	KabupatenKota   string  `json:"kabupaten_kota"`
	Provinsi        string  `json:"provinsi"`
	Religion        string  `json:"agama"`
	MaritalStatus   string  `json:"status_perkawinan"`
	Occupation      string  `json:"pekerjaan"`
	Nationality     string  `json:"kewarganegaraan"`
	ExpiryDate      string  `json:"berlaku_hingga"`
	BloodType       string  `json:"golongan_darah"`
	ConfidenceScore float64 `json:"-"` // tidak ada di response VIDA, default 0
}

// ─── Identity Verification / Fraud Mitigation ────────────────────────────────
// Endpoint: POST https://services-sandbox.vida.id/main/v2/services/fraud
// Pakai credential yang sama dengan OCR

type FraudRequest struct {
	PartnerTrxID    string       `json:"partnerTrxId"`
	GovID           string       `json:"govId"`     // NIK
	GovIDType       string       `json:"govIdType"` // "KTP"
	FullName        string       `json:"fullName"`
	DOB             string       `json:"dob"`    // "1990-06-13" (YYYY-MM-DD)
	Mobile          string       `json:"mobile"` // "+62821..."
	Email           string       `json:"email"`
	TransactionType string       `json:"transactionType"` // "FULL_FRAUD_ASSESSMENT"
	SelfiePhoto     string       `json:"selfiePhoto"`     // base64 tanpa data URI prefix
	Consent         FraudConsent `json:"consent"`         // wajib oleh VIDA API
}

// FraudConsent menyatakan persetujuan customer untuk verifikasi data ke Dukcapil.
// Wajib diisi — VIDA akan reject request tanpa field ini.
//
// Format aktual dari Postman collection VIDA:
//
//	"consent": {
//	    "consentedAt": "1681140102",  // Unix timestamp (detik) sebagai string
//	    "consentGiven": true
//	}
type FraudConsent struct {
	ConsentedAt  string `json:"consentedAt"`  // Unix timestamp (detik) sebagai string, e.g. "1681140102"
	ConsentGiven bool   `json:"consentGiven"` // true = customer menyetujui
}

// FraudSubmitResponse adalah response saat submit fraud mitigation
type FraudSubmitResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// FraudStatusResponse adalah response dari GET .../fraud/:id/status
type FraudStatusResponse struct {
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"` // "PROCESS" | "SUCCESS" | "FAILED"
	Data          FraudData `json:"data"`
}

// FraudData berisi hasil verifikasi identitas
type FraudData struct {
	// Hasil face match
	FaceMatch      bool    `json:"face_match"`
	FaceMatchScore float64 `json:"face_match_score"`

	// Hasil verifikasi ke Dukcapil
	DukcapilMatch bool   `json:"dukcapil_match"`
	RiskLevel     string `json:"risk_level"` // "LOW" | "MEDIUM" | "HIGH"
	Decision      string `json:"decision"`   // "ACCEPT" | "REJECT" | "REVIEW"
}

// ─── eMeterai ─────────────────────────────────────────────────────────────────
// SSO: https://qa-sso-26.np.vida.id/...
// Base: https://sandbox-stamp-gateway.np.vida.id
// Header tambahan: X-PARTNER-ID

type eMateraiUploadRequest struct {
	CodeDocument string `json:"codeDocument"` // "VD001"
	Page         int    `json:"page"`         // halaman meterai (1-indexed)
	LLX          int    `json:"llx"`          // koordinat X (dari kiri)
	LLY          int    `json:"lly"`          // koordinat Y (dari bawah)
	RefNum       string `json:"refNum"`       // unique reference, kita pakai application_id
	Document     string `json:"document"`     // base64 PDF (tanpa data URI prefix)
}

type eMateraiUploadResponse struct {
	RefNum  string `json:"refNum"`
	Status  string `json:"status"` // "PROCESS" | "SUCCESS" | "FAILED"
	Message string `json:"message"`
}

type eMateraiStatusResponse struct {
	RefNum       string `json:"refNum"`
	Status       string `json:"status"`       // "SUCCESS" | "FAILED" | "PROCESS"
	StampedDoc   string `json:"stampedDoc"`   // base64 PDF hasil stempel
	SerialNumber string `json:"serialNumber"` // nomor seri meterai
	MateraiID    string `json:"materaiId"`
}

// ─── PoA Digital Signing ──────────────────────────────────────────────────────
// Endpoint: POST /signer/v2/services/esign/poa?docType=nontemplate

type SignRequest struct {
	PartnerTrxID string        `json:"partnerTrxId"`
	Signer       SignerInfo    `json:"signer"`
	RequestInfo  RequestInfo   `json:"requestInfo"`
	Device       DeviceInfo    `json:"device"`
	SigningInfo  []SigningInfo `json:"signingInfo"`
}

type SignerInfo struct {
	Email  string `json:"email"`
	KeyID  string `json:"keyId"`  // POA ID dari email VIDA
	APIKey string `json:"apiKey"` // dari .env VIDA_SIGN_API_KEY
	EncCVV string `json:"encCVV"` // AES/CFB encrypt(CVV+epoch, secretKey, apiKey)
}

type RequestInfo struct {
	UserAgent        string `json:"userAgent"`
	SrcIP            string `json:"srcIp"`
	ConsentTimestamp string `json:"consentTimestamp"` // Unix timestamp string
}

type DeviceInfo struct {
	OS              string `json:"os"`
	Model           string `json:"model"`
	UniqueID        string `json:"uniqueId"`
	NetworkProvider string `json:"networkProvider"`
}

type SigningInfo struct {
	PDFFile        string          `json:"pdfFile"` // base64 PDF
	PageNo         string          `json:"pageNo"`
	XPoint         string          `json:"xPoint"`
	YPoint         string          `json:"yPoint"`
	Height         string          `json:"height"`
	Width          string          `json:"width"`
	ClientFilename string          `json:"clientFilename,omitempty"`
	QREnabled      bool            `json:"qrEnable,omitempty"`
	Appearance     *SignAppearance `json:"appearance,omitempty"`
}

type SignAppearance struct {
	Type      string `json:"type"`                // "standard" | "provided" | "non_visual"
	SignImage string `json:"signImage,omitempty"` // base64 image jika type="provided"
}

// SignResponse adalah response dari PoA sign endpoint
type SignResponse struct {
	Data struct {
		ID string `json:"id"` // transaction ID untuk cek status
	} `json:"data"`
}

// SignStatusResponse adalah response dari GET /signer/v2/services/esign/:id
type SignStatusResponse struct {
	Data SignStatusData `json:"data"`
}

type SignStatusData struct {
	ID              string `json:"id"`
	Status          string `json:"status"`          // "COMPLETED" | "PENDING" | "FAILED"
	SignedDocBase64 string `json:"signedDocBase64"` // PDF yang sudah ditandatangani
}

// ─── Generic Error ────────────────────────────────────────────────────────────

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Errors  []struct {
		Code   int    `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors,omitempty"`
}
