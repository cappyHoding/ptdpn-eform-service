// Package ioh provides SMS sending via IOH (Indosat Ooredoo Hutchison) Bulk SMS Gateway.
// Docs: Panduan_via_API__IMS.pdf
//
// API: POST https://smsapi.three.co.id:25000/sendsms
// Auth: username + password (diberikan oleh IOH account manager)
// Format nomor: 628xxx (bukan 08xxx atau +62xxx)
// Response: "errorcode|BALANCE:n|COUNT:n|TRANSACTIONID:xxx"
package ioh

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultSMSURL = "https://smsapi.three.co.id:25000/sendsms"

// SMSClient mengirim SMS via IOH Bulk SMS Gateway.
type SMSClient struct {
	username   string
	password   string
	senderID   string // msisdn_sender — brand name di HP penerima, contoh: "BPRPERDANA"
	apiURL     string
	httpClient *http.Client
}

// NewSMSClient membuat SMSClient baru.
// senderID adalah brand name yang akan muncul sebagai pengirim di HP nasabah.
func NewSMSClient(username, password, senderID string) *SMSClient {
	return &SMSClient{
		username:   username,
		password:   password,
		senderID:   senderID,
		apiURL:     defaultSMSURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// SMSResult adalah parsed response dari IOH.
type SMSResult struct {
	ErrorCode     string // "6801" = sukses
	Balance       string // sisa balance setelah pengiriman
	Count         string // jumlah token yang dikurangi
	TransactionID string // ID transaksi IOH untuk tracking
}

// IsSuccess mengembalikan true jika SMS berhasil dikirim.
// 6801 = delivered ke network operator
// 6804 = error dari operator tapi masih charged (treated as partial success)
func (r *SMSResult) IsSuccess() bool {
	return r.ErrorCode == "6801"
}

// Send mengirim SMS ke satu nomor tujuan.
//
// msisdn HARUS dalam format 628xxx — gunakan NormalizePhone() jika perlu.
// referenceID maksimal 64 karakter, dipakai untuk tracking di laporan IOH.
func (c *SMSClient) Send(msisdn, message, referenceID string) (*SMSResult, error) {
	if msisdn == "" {
		return nil, fmt.Errorf("msisdn cannot be empty")
	}
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}
	if len(referenceID) > 64 {
		referenceID = referenceID[:64]
	}

	params := url.Values{}
	params.Set("username", c.username)
	params.Set("password", c.password)
	params.Set("msisdn", msisdn)
	params.Set("msisdn_sender", c.senderID)
	params.Set("message", message)
	params.Set("referenceid", referenceID)

	resp, err := c.httpClient.Post(
		c.apiURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("IOH SMS HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	result, err := parseIOHResponse(strings.TrimSpace(string(body)))
	if err != nil {
		return nil, err
	}

	// Error codes dari docs IOH
	switch result.ErrorCode {
	case "6801":
		return result, nil // sukses
	case "6805":
		return result, fmt.Errorf("IOH: wrong credential or IP not whitelisted (6805)")
	case "6806":
		return result, fmt.Errorf("IOH: unsupported prefix or unknown destination operator (6806)")
	case "6808":
		return result, fmt.Errorf("IOH: insufficient balance (6808)")
	case "6809":
		return result, fmt.Errorf("IOH: unregistered sender ID (6809)")
	case "6011":
		return result, fmt.Errorf("IOH: OTP message rejected by content filter — need OTP account (6011)")
	default:
		return result, fmt.Errorf("IOH error code %s", result.ErrorCode)
	}
}

// NormalizePhone mengkonversi nomor HP ke format IOH (628xxx).
func NormalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+62") {
		return phone[1:] // hapus "+"
	}
	if strings.HasPrefix(phone, "0") {
		return "62" + phone[1:] // ganti 0 → 62
	}
	return phone
}

// parseIOHResponse mem-parse response format IOH:
// "6801|BALANCE:999|COUNT:1|TRANSACTIONID:abc123"
func parseIOHResponse(raw string) (*SMSResult, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty IOH response")
	}
	parts := strings.Split(raw, "|")
	result := &SMSResult{ErrorCode: parts[0]}
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "BALANCE":
			result.Balance = kv[1]
		case "COUNT":
			result.Count = kv[1]
		case "TRANSACTIONID":
			result.TransactionID = kv[1]
		}
	}
	return result, nil
}
