package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/integration/ioh"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type OTPService interface {
	SendOTP(ctx context.Context, appID, phone string) error
	VerifyOTP(ctx context.Context, appID, code string) error
}

const (
	otpTTL         = 5 * time.Minute
	otpCooldownTTL = 60 * time.Second
	otpMaxAttempts = 3
	otpLockoutTTL  = 15 * time.Minute
)

type otpService struct {
	sms *ioh.SMSClient
	rdb *redis.Client
	log *logger.Logger
}

func NewOTPService(sms *ioh.SMSClient, rdb *redis.Client, log *logger.Logger) OTPService {
	return &otpService{sms: sms, rdb: rdb, log: log}
}

func (s *otpService) SendOTP(ctx context.Context, appID, phone string) error {
	// Cek cooldown
	cooldownKey := "otp:cooldown:" + appID
	ttl, _ := s.rdb.TTL(ctx, cooldownKey).Result()
	if ttl > 0 {
		return fmt.Errorf("OTP_COOLDOWN:%d", int(ttl.Seconds()))
	}

	// Generate 4-digit OTP secara kriptografis aman
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return fmt.Errorf("generate OTP failed: %w", err)
	}
	code := fmt.Sprintf("%04d", n.Int64()+1000) // 1000–9999

	// Simpan ke Redis
	codeKey := "otp:code:" + appID
	if err := s.rdb.Set(ctx, codeKey, code, otpTTL).Err(); err != nil {
		return fmt.Errorf("save OTP to Redis failed: %w", err)
	}

	// Set cooldown + reset attempt counter
	s.rdb.Set(ctx, cooldownKey, "1", otpCooldownTTL)
	s.rdb.Del(ctx, "otp:attempts:"+appID)

	// Kirim SMS
	msisdn := ioh.NormalizePhone(phone)
	message := fmt.Sprintf(
		"BPR Perdana: Kode OTP Anda adalah %s. Berlaku 5 menit. Jangan bagikan kode ini kepada siapapun.",
		code,
	)
	refID := "OTP-" + appID[:8]

	result, err := s.sms.Send(msisdn, message, refID)
	if err != nil {
		s.log.Error("OTP SMS send failed",
			zap.String("app_id", appID),
			zap.String("phone", maskPhone(phone)),
			zap.Error(err),
		)
		return fmt.Errorf("SMS_SEND_FAILED: %w", err)
	}

	s.log.Info("OTP SMS sent",
		zap.String("app_id", appID),
		zap.String("phone", maskPhone(phone)),
		zap.String("ioh_trx", result.TransactionID),
	)
	return nil
}

func (s *otpService) VerifyOTP(ctx context.Context, appID, inputCode string) error {
	// Cek lockout
	attemptsKey := "otp:attempts:" + appID
	attempts, _ := s.rdb.Get(ctx, attemptsKey).Int()
	if attempts >= otpMaxAttempts {
		ttl, _ := s.rdb.TTL(ctx, attemptsKey).Result()
		return fmt.Errorf("OTP_LOCKED:%d", int(ttl.Seconds()))
	}

	// Ambil kode dari Redis
	codeKey := "otp:code:" + appID
	storedCode, err := s.rdb.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return fmt.Errorf("OTP_EXPIRED")
	}
	if err != nil {
		return fmt.Errorf("Redis error: %w", err)
	}

	// Bandingkan
	if storedCode != inputCode {
		pipe := s.rdb.Pipeline()
		pipe.Incr(ctx, attemptsKey)
		pipe.Expire(ctx, attemptsKey, otpLockoutTTL)
		pipe.Exec(ctx)
		remaining := otpMaxAttempts - (attempts + 1)
		return fmt.Errorf("OTP_INVALID:%d", remaining)
	}

	// Berhasil
	s.rdb.Del(ctx, codeKey, attemptsKey, "otp:cooldown:"+appID)
	s.log.Info("OTP verified", zap.String("app_id", appID))
	return nil
}

func maskPhone(phone string) string {
	if len(phone) <= 6 {
		return "****"
	}
	return phone[:3] + strings.Repeat("*", len(phone)-6) + phone[len(phone)-3:]
}
