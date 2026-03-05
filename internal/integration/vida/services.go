package vida

import (
	"fmt"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/config"
)

// Services adalah registry yang menggabungkan semua VIDA service.
//
// ARSITEKTUR CLIENT:
//
//	OCR + Fraud Mitigation → satu client (credential sama, base URL sama)
//	PoA Sign               → client terpisah (credential berbeda)
//	eMeterai               → client terpisah (SSO berbeda, base URL berbeda)
//
// CARA PAKAI:
//
//	vida := vida.NewServices(cfg)
//	vida.OCR.ExtractKTP(ctx, imageReader, "ktp.jpg")
//	vida.Fraud.VerifyIdentity(ctx, selfieB64, nik, name, dob, phone, email)
//	vida.Sign.RegisterDocument(ctx, pdfPath, email, filename, srcIP, ua)
//	vida.EMeterai.ApplyStamp(ctx, pdfPath, refNum)
type Services struct {
	OCR      *OCRService
	Fraud    *FraudService
	Sign     *SignService
	EMeterai *eMateraiService
}

// NewServices membuat semua VIDA service dari konfigurasi.
// Dipanggil sekali saat startup di main.go.
func NewServices(cfg *config.Config) *Services {
	timeout := cfg.Vida.HTTPTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// ── OCR + Fraud Mitigation: credential dan base URL yang sama ─────────────
	const ocrFraudSSO = "https://qa-sso.vida.id/auth/realms/vida/protocol/openid-connect/token"
	ocrFraudClient := NewClient(
		cfg.Vida.OCR.BaseURL, // "https://services-sandbox.vida.id"
		ocrFraudSSO,
		cfg.Vida.OCR.ClientID,  // "partner-bpr-perdana-snbx-sso"
		cfg.Vida.OCR.SecretKey, // "JDm1jOqtcvw5YRjakRvoWw4pG1MK40t0"
		"roles",
		timeout,
	)

	// ── PoA Sign: credential berbeda ─────────────────────────────────────────
	const signSSO = "https://qa-sso.vida.id/auth/realms/vida/protocol/openid-connect/token"
	signClient := NewClient(
		cfg.Vida.Sign.BaseURL, // "https://services-sandbox.vida.id"
		signSSO,
		cfg.Vida.Sign.ClientID,  // "partner-bprperdana-snbx-poa"
		cfg.Vida.Sign.SecretKey, // "2wAYiYR8TF2ktIWtjIg3RSZDObTBF7Pc"
		"roles",
		timeout,
	)

	// ── eMeterai: SSO dan base URL berbeda ────────────────────────────────────
	const eMateraiSSO = "https://qa-sso-26.np.vida.id/auth/realms/vida/protocol/openid-connect/token"
	eMateraiClient := NewClient(
		cfg.Vida.EMaterai.BaseURL, // "https://sandbox-stamp-gateway.np.vida.id"
		eMateraiSSO,
		cfg.Vida.EMaterai.ClientID,  // "partner-bprperdana-emeterai-snbx-sso"
		cfg.Vida.EMaterai.SecretKey, // "VsTbprihsJiPHJwJgGQrARNz8lHro4zl"
		"openid",                    // scope berbeda: "openid" bukan "roles"
		timeout,
	)

	// Validasi kredensial Sign service saat startup
	if len(cfg.Vida.Sign.CVV) != 3 {
		panic(fmt.Sprintf("VIDA_SIGN_CVV harus tepat 3 digit, got %d chars: %q", len(cfg.Vida.Sign.CVV), cfg.Vida.Sign.CVV))
	}
	if len(cfg.Vida.Sign.APIKey) != 16 {
		panic(fmt.Sprintf("VIDA_SIGN_API_KEY harus tepat 16 chars, got %d chars", len(cfg.Vida.Sign.APIKey)))
	}
	// CVVSecretKey harus 64 hex chars (32 bytes AES-256)
	if len(cfg.Vida.Sign.CVVSecretKey) != 64 {
		panic(fmt.Sprintf(
			"VIDA_SIGN_CVV_SECRET_KEY harus 64 hex chars (32 bytes AES-256), got %d chars",
			len(cfg.Vida.Sign.CVVSecretKey),
		))
	}

	return &Services{
		OCR:   NewOCRService(ocrFraudClient),
		Fraud: NewFraudService(ocrFraudClient), // share client yang sama dengan OCR
		Sign: NewSignService(
			signClient,
			cfg.Vida.Sign.CVV,          // 3 digit CVV dari VIDA
			cfg.Vida.Sign.CVVSecretKey, // 64 hex chars, khusus enkripsi CVV
			cfg.Vida.Sign.APIKey,       // 16 chars — IV untuk enkripsi CVV
			cfg.Vida.Sign.KeyID,        // POA ID dari email VIDA
		),
		EMeterai: NeweMateraiService(eMateraiClient, cfg.Vida.EMaterai.PartnerID),
	}
}
