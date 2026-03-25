package vida

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FraudService menangani Identity Verification menggunakan VIDA Fraud Mitigation API.
type FraudService struct {
	client *Client
	logger *zap.Logger
}

func NewFraudService(client *Client) *FraudService {
	logger, _ := zap.NewProduction()
	return &FraudService{client: client, logger: logger}
}

// VerifyIdentity mengirim selfie + data KTP ke VIDA untuk verifikasi identitas.
//
// PENTING: VIDA Fraud API bersifat SYNCHRONOUS — hasil assessment sudah tersedia
// langsung di response submit (field assessmentResults + fraudAssessment).
// Polling ke /status hanya mengembalikan status transaksi ("002" = submitted),
// bukan hasil assessment. Kita tidak perlu polling.
func (s *FraudService) VerifyIdentity(
	ctx context.Context,
	selfieB64, nik, fullName, dob, mobile, email string,
) (*FraudData, error) {
	partnerTrxID := uuid.New().String()
	consentedAt := fmt.Sprintf("%d", time.Now().Unix())
	selfieClean := selfieB64
	if idx := strings.Index(selfieB64, ","); idx != -1 {
		selfieClean = selfieB64[idx+1:]
	}

	isSandbox := strings.Contains(s.client.baseURL, "sandbox") ||
		strings.Contains(s.client.baseURL, "np.vida") ||
		strings.Contains(s.client.ssoURL, "qa-sso")

	if isSandbox {
		s.logger.Info("Using VIDA sandbox mock data for fraud assessment",
			zap.String("original_nik", nik),
			zap.String("mock_nik", "3511000101806300"),
		)
		nik = "3511000101806300"
		fullName = "UserGDAA"
		dob = "1980-01-01"
	}

	// Tangkap response mentah untuk ekstrak assessmentResults
	var rawResp map[string]interface{}
	err := s.client.doJSON(ctx, http.MethodPost,
		"/main/v2/services/fraud",
		FraudRequest{
			PartnerTrxID:    partnerTrxID,
			GovID:           nik,
			GovIDType:       "KTP",
			FullName:        fullName,
			DOB:             dob,
			Mobile:          mobile,
			Email:           email,
			TransactionType: "FULL_FRAUD_ASSESSMENT",
			SelfiePhoto:     selfieClean,
			Consent: FraudConsent{
				ConsentedAt:  consentedAt,
				ConsentGiven: true,
			},
		},
		&rawResp,
	)

	if err != nil {
		return nil, fmt.Errorf("fraud mitigation submit failed: %w", err)
	}

	rawJSON, _ := json.Marshal(rawResp)
	s.logger.Info("VIDA fraud submit response", zap.String("body", string(rawJSON)))

	// Ekstrak transactionId untuk logging/audit
	transactionID := extractTransactionID(rawResp)
	s.logger.Info("Fraud assessment submitted",
		zap.String("transaction_id", transactionID),
		zap.String("partner_trx_id", partnerTrxID),
	)

	// Ekstrak hasil assessment dari submit response (tidak perlu polling)
	result := extractFraudDataFromSubmit(rawResp)

	s.logger.Info("Fraud assessment result",
		zap.String("transaction_id", transactionID),
		zap.String("decision", result.Decision),
		zap.Float64("selfie_score", result.FaceMatchScore),
		zap.Bool("face_match", result.FaceMatch),
		zap.Bool("nik_match", result.DukcapilMatch),
	)

	return result, nil
}

// extractFraudDataFromSubmit mengekstrak hasil dari response submit VIDA.
//
// Struktur response aktual VIDA:
//
//	{
//	  "data": {
//	    "assessmentResults": [
//	      {"name": "full_name",   "result": 1},     // 1 = match, 0 = no match
//	      {"name": "dob",         "result": 0},
//	      {"name": "nik",         "result": 1},
//	      {"name": "selfiePhoto", "result": 0.9}    // similarity score 0-1
//	    ],
//	    "fraudAssessment": "SUBMITTED",
//	    "transactionId": "...",
//	    "transactionType": "FULL_FRAUD_ASSESSMENT"
//	  }
//	}
func extractFraudDataFromSubmit(resp map[string]interface{}) *FraudData {
	result := &FraudData{
		Decision:  "REVIEW",
		RiskLevel: "MEDIUM",
	}

	// Masuk ke "data"
	dataMap, ok := getDataMap(resp)
	if !ok {
		return result
	}

	// Parse assessmentResults array
	assessments, _ := dataMap["assessmentResults"].([]interface{})
	for _, item := range assessments {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		scoreVal := entry["result"]

		switch name {
		case "selfiePhoto":
			// Similarity score 0.0–1.0
			if score, ok := toFloat64(scoreVal); ok {
				result.FaceMatchScore = score
				result.FaceMatch = score >= 0.7
			}
		case "nik":
			// 1 = NIK cocok dengan Dukcapil
			if score, ok := toFloat64(scoreVal); ok {
				result.DukcapilMatch = score >= 1.0
			}
		}
	}

	// Tentukan Decision berdasarkan hasil assessment
	result.Decision = determineDecision(result.FaceMatch, result.DukcapilMatch, result.FaceMatchScore)
	result.RiskLevel = determineRiskLevel(result.FaceMatch, result.DukcapilMatch)

	return result
}

// determineDecision menentukan keputusan akhir berdasarkan hasil assessment.
//
// Logika bisnis BPR Perdana:
//   - NIK harus cocok dengan Dukcapil (wajib untuk KYC perbankan)
//   - Selfie similarity >= 0.7 dianggap match
//   - Keduanya harus pass untuk ACCEPT
func determineDecision(faceMatch, dukcapilMatch bool, faceScore float64) string {
	if dukcapilMatch && faceMatch {
		return "ACCEPT"
	}
	if !dukcapilMatch {
		// NIK tidak cocok = langsung reject
		return "REJECT"
	}
	// NIK cocok tapi selfie kurang — perlu review manual
	return "REVIEW"
}

func determineRiskLevel(faceMatch, dukcapilMatch bool) string {
	if dukcapilMatch && faceMatch {
		return "LOW"
	}
	if !dukcapilMatch {
		return "HIGH"
	}
	return "MEDIUM"
}

// ── Helper functions ──────────────────────────────────────────────────────────

func getDataMap(resp map[string]interface{}) (map[string]interface{}, bool) {
	if data, ok := resp["data"]; ok {
		if dm, ok := data.(map[string]interface{}); ok {
			return dm, true
		}
	}
	return resp, true // fallback ke root jika tidak ada "data"
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

// extractTransactionID mencoba berbagai kemungkinan struktur response VIDA.
func extractTransactionID(resp map[string]interface{}) string {
	for _, key := range []string{"transaction_id", "transactionId", "trxId", "id"} {
		if v, ok := resp[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if data, ok := resp["data"]; ok {
		if dm, ok := data.(map[string]interface{}); ok {
			for _, key := range []string{"transaction_id", "transactionId", "trxId", "id"} {
				if v, ok := dm[key]; ok {
					if s, ok := v.(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

// Pastikan time tidak unused jika tidak ada call lain
var _ = time.Second
