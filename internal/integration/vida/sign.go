package vida

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// SignService menangani PoA Digital Signing menggunakan VIDA API.
//
// ALUR:
//  1. Generate encCVV dari CVV + epoch menggunakan AES/CFB
//  2. POST ke /signer/v2/services/esign/poa?docType=nontemplate
//  3. Poll GET /signer/v2/services/esign/:id sampai COMPLETED
//  4. Download dan simpan PDF yang sudah ditandatangani
//
// encCVV GENERATION:
//
//	plaintext  = CVV + epochSeconds (e.g. "049" + "1706605635" = "0491706605635")
//	key        = secretKey di-parse dari hex string ke bytes (32 bytes)
//	IV         = apiKey sebagai UTF-8 bytes (16 bytes)
//	algorithm  = AES/CFB/NoPadding
//	output     = URL-safe base64 dari ciphertext
type SignService struct {
	client       *Client
	cvv          string // clear text CVV 3 digit (VIDA_SIGN_CVV)
	cvvSecretKey string // 64 hex chars khusus enkripsi CVV (VIDA_SIGN_CVV_SECRET_KEY)
	apiKey       string // 16 chars, dipakai sebagai IV untuk enkripsi CVV
	keyID        string // POA ID (poaId) dari VIDA
}

func NewSignService(client *Client, cvv, cvvSecretKey, apiKey, keyID string) *SignService {
	return &SignService{
		client:       client,
		cvv:          cvv,
		cvvSecretKey: cvvSecretKey,
		apiKey:       apiKey,
		keyID:        keyID,
	}
}

// RegisterDocument mendaftarkan PDF ke VIDA untuk ditandatangani oleh customer.
//
// Parameters:
//   - ctx:            context
//   - pdfPath:        path ke PDF yang sudah di-stempel meterai
//   - signerEmail:    email customer yang akan menandatangani
//   - clientFilename: nama dokumen yang ditampilkan ke customer
//   - srcIP:          IP address customer (dari request context)
//   - userAgent:      user agent browser customer
//
// Returns:
//   - transactionID: ID untuk polling status
func (s *SignService) RegisterDocument(
	ctx context.Context,
	pdfPath, signerEmail, clientFilename, srcIP, userAgent string,
) (string, error) {
	// ── 1. Baca dan encode PDF ────────────────────────────────────────────────
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF for signing: %w", err)
	}
	pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	// ── 2. Generate encCVV ────────────────────────────────────────────────────
	encCVV, err := s.generateEncCVV()
	if err != nil {
		return "", fmt.Errorf("failed to generate encCVV: %w", err)
	}

	// ── 3. Build request ──────────────────────────────────────────────────────
	req := SignRequest{
		PartnerTrxID: uuid.New().String(),
		Signer: SignerInfo{
			Email:  signerEmail,
			KeyID:  s.keyID,
			APIKey: s.apiKey,
			EncCVV: encCVV,
		},
		RequestInfo: RequestInfo{
			UserAgent:        userAgent,
			SrcIP:            srcIP,
			ConsentTimestamp: strconv.FormatInt(time.Now().Unix(), 10),
		},
		// Device info: gunakan nilai default karena ini server-side call
		// bukan request langsung dari device customer
		Device: DeviceInfo{
			OS:              "Server",
			Model:           "BPR-Perdana-Backend",
			UniqueID:        uuid.New().String(),
			NetworkProvider: "Internet",
		},
		SigningInfo: []SigningInfo{
			{
				PDFFile:        pdfBase64,
				PageNo:         "1",
				XPoint:         "80",
				YPoint:         "100",
				Height:         "50",
				Width:          "200",
				ClientFilename: clientFilename,
				QREnabled:      true,
				Appearance: &SignAppearance{
					Type: "standard", // pakai logo VIDA standar
				},
			},
		},
	}

	// ── 4. Submit ke VIDA ─────────────────────────────────────────────────────
	var signResp SignResponse
	err = s.client.doJSON(ctx, http.MethodPost,
		"/signer/v2/services/esign/poa?docType=nontemplate",
		req,
		&signResp,
	)
	if err != nil {
		return "", fmt.Errorf("PoA sign registration failed: %w", err)
	}

	if signResp.Data.ID == "" {
		return "", fmt.Errorf("PoA sign returned empty transaction ID")
	}

	return signResp.Data.ID, nil
}

// GetSignedDocument polling status dan mengembalikan PDF yang sudah ditandatangani.
//
// Parameters:
//   - ctx:           context dengan timeout (rekomendasi: 5 menit — customer perlu waktu untuk sign)
//   - transactionID: ID dari RegisterDocument
func (s *SignService) GetSignedDocument(ctx context.Context, transactionID string) (*SignStatusData, error) {
	var finalResp SignStatusResponse
	statusPath := fmt.Sprintf("/signer/v2/services/esign/%s", transactionID)

	err := s.client.PollUntilDone(ctx, statusPath,
		func(body []byte) bool {
			var resp SignStatusResponse
			if parseErr := parseJSONStatus(body, &resp); parseErr != nil {
				return false
			}
			return resp.Data.Status == "COMPLETED" || resp.Data.Status == "FAILED"
		},
		&finalResp,
		10*pollInterval, // sign butuh customer action, poll tiap 10 detik
		30,              // max 5 menit
	)
	if err != nil {
		return nil, fmt.Errorf("sign polling failed (transaction: %s): %w", transactionID, err)
	}

	if finalResp.Data.Status == "FAILED" {
		return nil, fmt.Errorf("PoA signing failed for transaction %s", transactionID)
	}

	return &finalResp.Data, nil
}

// generateEncCVV membuat encrypted CVV menggunakan AES/CFB128/NoPadding.
//
// ALGORITMA (sesuai dokumentasi VIDA — Java ExampleAES.java):
//
//	plaintext = CVV + epochSeconds   e.g. "820" + "1706605635" = "8201706605635"
//	key       = secretKey di-decode dari hex (32 bytes untuk AES-256)
//	IV        = apiKey sebagai UTF-8 bytes (tepat 16 bytes)
//	cipher    = AES/CFB/NoPadding (CFB128 — full block segment, bukan CFB8)
//	leftPad   = NO-OP (Java logic: target selalu <= input length)
//	output    = URL-safe base64 (Java Base64.getUrlEncoder(), dengan = padding)
//
// PENTING — perbedaan Go vs Java:
//
//	Go cipher.NewCFBEncrypter = CFB8  (8-bit segment) → SALAH
//	Java AES/CFB/NoPadding    = CFB128 (128-bit segment) → BENAR
//	Kita implementasi CFB128 manual menggunakan cipher.NewCBCEncrypter trick.
func (s *SignService) generateEncCVV() (string, error) {
	// Plaintext: CVV + epoch seconds saat ini (NO padding — Java leftPad is a no-op)
	epochNow := strconv.FormatInt(time.Now().Unix(), 10)
	plaintext := []byte(s.cvv + epochNow)

	// Decode CVVSecretKey dari hex ke bytes (AES-256, 64 hex chars = 32 bytes)
	keyBytes, err := hexToBytes(s.cvvSecretKey)
	if err != nil {
		return "", fmt.Errorf("invalid CVV secretKey: %w", err)
	}

	// IV = apiKey sebagai bytes (harus tepat 16 bytes)
	iv := []byte(s.apiKey)
	if len(iv) != 16 {
		return "", fmt.Errorf("apiKey must be exactly 16 bytes, got %d", len(iv))
	}

	// Enkripsi menggunakan CFB128 (sama dengan Java AES/CFB/NoPadding)
	ciphertext, err := encryptAESCFB128(keyBytes, iv, plaintext)
	if err != nil {
		return "", fmt.Errorf("AES CFB128 encryption failed: %w", err)
	}

	// URL-safe base64 WITH padding — Java Base64.getUrlEncoder() menyertakan '='
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// encryptAESCFB128 mengenkripsi data menggunakan AES-CFB dengan segment size 128-bit.
//
// Java AES/CFB/NoPadding menggunakan CFB128 (segment = full AES block = 16 bytes).
// Go stdlib hanya menyediakan CFB8 via cipher.NewCFBEncrypter.
// Kita implementasi CFB128 manual:
//
//	keystream[0] = AES_Encrypt(IV)
//	C[i] = P[i] XOR keystream[i % 16]  (proses byte per byte, keystream di-refresh tiap 16 bytes)
//
// Karena plaintext < 16 bytes (CVV+epoch = 13 bytes), hanya 1 block keystream yang dibutuhkan.
func encryptAESCFB128(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Proses dalam chunks 16-byte (CFB128)
	ciphertext := make([]byte, len(plaintext))
	blockSize := block.BlockSize() // 16

	// feedback = IV awal
	feedback := make([]byte, blockSize)
	copy(feedback, iv)

	keystreamBlock := make([]byte, blockSize)

	for i := 0; i < len(plaintext); {
		// Generate keystream block = Encrypt(feedback)
		block.Encrypt(keystreamBlock, feedback)

		// XOR plaintext chunk dengan keystream
		chunkEnd := i + blockSize
		if chunkEnd > len(plaintext) {
			chunkEnd = len(plaintext)
		}
		for j := i; j < chunkEnd; j++ {
			ciphertext[j] = plaintext[j] ^ keystreamBlock[j-i]
		}

		// Update feedback = ciphertext block yang baru saja dihasilkan
		// Untuk CFB128: feedback = full 16-byte ciphertext block
		// Jika chunk < 16 bytes (last block), pad ciphertext dengan 0x00 untuk feedback
		copy(feedback, ciphertext[i:chunkEnd])
		if chunkEnd-i < blockSize {
			// Zero-fill sisa feedback (tidak dipakai karena ini block terakhir)
			for k := chunkEnd - i; k < blockSize; k++ {
				feedback[k] = 0
			}
		}

		i = chunkEnd
	}

	return ciphertext, nil
}

// javaHexDigit mengembalikan nilai digit hex seperti Java Character.digit(char, 16).
// Mengembalikan -1 untuk karakter non-hex — TIDAK error, sesuai perilaku Java.
// Ini penting karena VIDA server menggunakan fungsi Java yang sama untuk decrypt.
func javaHexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1 // non-hex char → -1, sama persis dengan Java Character.digit
	}
}

// hexToBytes adalah replika TEPAT dari Java hexStringToByteArray di VIDA docs.
//
// Java code:
//
//	data[i/2] = (byte)((Character.digit(s.charAt(i), 16) << 4)
//	                   + Character.digit(s.charAt(i+1), 16));
//
// KUNCI: Java Character.digit mengembalikan -1 untuk non-hex char (tanpa exception).
// Hasilnya: ((-1) << 4) + (-1) = -17 → byte 0xEF
// VIDA server menggunakan fungsi yang SAMA untuk decode secretKey sebelum decrypt.
// Kita HARUS mereplikasi perilaku ini agar hasil enkripsi cocok.
func hexToBytes(keyStr string) ([]byte, error) {
	if len(keyStr)%2 != 0 {
		return nil, fmt.Errorf("secretKey harus panjang genap, got %d chars", len(keyStr))
	}
	result := make([]byte, len(keyStr)/2)
	for i := 0; i < len(keyStr); i += 2 {
		high := javaHexDigit(keyStr[i])
		low := javaHexDigit(keyStr[i+1])
		// Java cast to byte: (byte)((high << 4) + low)
		// Jika high atau low = -1 (non-hex), hasilnya tetap dihitung
		val := (high << 4) + low
		result[i/2] = byte(val & 0xFF) // same as Java (byte) cast
	}
	return result, nil
}

// leftPad — dihapus: Java leftPad di VIDA docs adalah NO-OP.
// Proof: target = 16 - sizePad = 16 - (16 - len%16) = len%16 ≤ len
// → kondisi size <= data.length selalu true → selalu return data unchanged.

// TestGenerateEncCVV adalah exported helper untuk test script.
// Memverifikasi bahwa encCVV generation berfungsi dengan benar.
func (s *SignService) TestGenerateEncCVV() (string, error) {
	return s.generateEncCVV()
}
