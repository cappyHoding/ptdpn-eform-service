package vida

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
)

// eMateraiService menangani penerapan meterai elektronik ke PDF.
//
// PERBEDAAN DARI SERVICE LAIN:
//   - SSO endpoint berbeda: https://qa-sso-26.np.vida.id/...
//   - Base URL berbeda:     https://sandbox-stamp-gateway.np.vida.id
//   - Scope: "openid" (bukan "roles")
//   - Butuh header X-PARTNER-ID di setiap request
//   - PDF dikirim sebagai base64 TANPA data URI prefix
//
// FLOW:
//  1. Upload PDF base64 → dapat refNum
//  2. Poll GET /api/v1/emeterai/docstamp/:refNum sampai SUCCESS
//  3. Download PDF hasil stempel dari response
type eMateraiService struct {
	client    *Client
	partnerID string // X-PARTNER-ID header value
}

func NeweMateraiService(client *Client, partnerID string) *eMateraiService {
	return &eMateraiService{client: client, partnerID: partnerID}
}

// ApplyStamp menerapkan meterai elektronik ke PDF kontrak.
//
// Parameters:
//   - ctx:      context dengan timeout (rekomendasi: 60 detik)
//   - pdfPath:  path ke file PDF yang akan distempel
//   - refNum:   unique reference number (gunakan application_id)
//
// Returns:
//   - *eMateraiStatusResponse: berisi stampedDoc (base64 PDF) dan metadata
func (s *eMateraiService) ApplyStamp(ctx context.Context, pdfPath, refNum string) (*eMateraiStatusResponse, error) {
	// ── 1. Baca PDF dan encode ke base64 (tanpa prefix) ───────────────────────
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}
	pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	// ── 2. Upload PDF ke VIDA eMeterai ────────────────────────────────────────
	headers := map[string]string{
		"X-PARTNER-ID": s.partnerID,
	}

	var uploadResp eMateraiUploadResponse
	err = s.client.doJSONWithHeaders(ctx, http.MethodPost,
		"/api/v1/emeterai/upload",
		headers,
		eMateraiUploadRequest{
			CodeDocument: "VD001", // kode dokumen standar dari VIDA
			Page:         1,       // halaman pertama untuk meterai
			LLX:          100,     // koordinat X dari kiri (dalam points)
			LLY:          100,     // koordinat Y dari bawah (dalam points)
			RefNum:       refNum,  // application_id sebagai reference
			Document:     pdfBase64,
		},
		&uploadResp,
	)
	if err != nil {
		return nil, fmt.Errorf("eMeterai upload failed: %w", err)
	}

	// ── 3. Poll status sampai PDF selesai distempel ───────────────────────────
	statusPath := fmt.Sprintf("/api/v1/emeterai/docstamp/%s", refNum)

	var finalResp eMateraiStatusResponse
	err = s.client.PollUntilDone(ctx, statusPath,
		func(body []byte) bool {
			var resp eMateraiStatusResponse
			if parseErr := parseJSONStatus(body, &resp); parseErr != nil {
				return false
			}
			return resp.Status == "SUCCESS" || resp.Status == "FAILED"
		},
		&finalResp,
		3*pollInterval, // eMeterai sedikit lebih lambat, interval 3 detik
		20,
	)
	if err != nil {
		return nil, fmt.Errorf("eMeterai polling failed (refNum: %s): %w", refNum, err)
	}

	if finalResp.Status == "FAILED" {
		return nil, fmt.Errorf("VIDA eMeterai failed for refNum %s", refNum)
	}

	return &finalResp, nil
}

// GetQuota mengecek sisa kuota meterai yang tersedia.
// Useful untuk monitoring agar tidak kehabisan kuota tiba-tiba.
func (s *eMateraiService) GetQuota(ctx context.Context) (map[string]interface{}, error) {
	headers := map[string]string{
		"X-PARTNER-ID": s.partnerID,
	}
	var result map[string]interface{}
	err := s.client.doJSONWithHeaders(ctx, http.MethodGet,
		"/api/v1/emeterai/quota",
		headers, nil, &result,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get eMeterai quota: %w", err)
	}
	return result, nil
}
