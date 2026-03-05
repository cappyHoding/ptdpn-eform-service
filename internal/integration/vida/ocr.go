package vida

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OCRService struct {
	client *Client
}

func NewOCRService(client *Client) *OCRService {
	return &OCRService{client: client}
}

// ExtractKTP mengirim gambar KTP ke VIDA OCR API dan mengembalikan data yang diekstrak.
//
// CATATAN: VIDA OCR sandbox mengembalikan hasil LANGSUNG di response submit.
// Tidak perlu polling — data sudah ada di response pertama.
//
// Struktur response aktual:
//
//	{
//	  "data": {
//	    "result": { "nik": "...", "nama": "...", "alamat": "...", ... },
//	    "status": "success",
//	    "transaction_id": "uuid"
//	  }
//	}
func (s *OCRService) ExtractKTP(ctx context.Context, imageReader io.Reader, filename string) (*OCRData, error) {
	// ── 1. Baca dan encode image ke base64 dengan data URI prefix ─────────────
	imageBytes, err := io.ReadAll(imageReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read KTP image: %w", err)
	}

	mimeType := detectImageMIME(filename)
	b64Image := fmt.Sprintf("data:%s;base64,%s",
		mimeType,
		base64.StdEncoding.EncodeToString(imageBytes),
	)

	// ── 2. Fetch token ────────────────────────────────────────────────────────
	token, err := s.client.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("OCR token failed: %w", err)
	}

	// ── 3. Build dan kirim request ────────────────────────────────────────────
	reqBody, err := json.Marshal(OCRRequest{
		Parameters: OCRParameters{KTPImage: b64Image},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OCR request: %w", err)
	}

	// Debug: log ukuran untuk deteksi image terlalu besar
	fmt.Printf("[VIDA OCR DEBUG] image_bytes=%d b64_len=%d request_body_len=%d\n",
		len(imageBytes), len(b64Image), len(reqBody))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.client.baseURL+"/verify/v1/ktp/ocr/transaction",
		strings.NewReader(string(reqBody)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCR request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	httpResp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OCR request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// ── 4. Baca dan parse response ────────────────────────────────────────────
	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("OCR returned HTTP %d: %s", httpResp.StatusCode, string(rawBody))
	}

	var resp OCRResponse
	if err := json.Unmarshal(rawBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse OCR response: %w\nraw: %s", err, string(rawBody))
	}

	// ── 5. Validasi hasil ─────────────────────────────────────────────────────
	if resp.Data.Status != "success" {
		return nil, fmt.Errorf("VIDA OCR status=%s (transaction: %s)",
			resp.Data.Status, resp.Data.TransactionID)
	}

	if resp.Data.Result.NIK == "" {
		return nil, fmt.Errorf("VIDA OCR succeeded but NIK is empty (transaction: %s)",
			resp.Data.TransactionID)
	}

	// ── 6. Set TransactionID ke result ────────────────────────────────────────
	result := resp.Data.Result
	result.TransactionID = resp.Data.TransactionID

	return &result, nil
}

func detectImageMIME(filename string) string {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".png") {
		return "image/png"
	}
	return "image/jpeg"
}
