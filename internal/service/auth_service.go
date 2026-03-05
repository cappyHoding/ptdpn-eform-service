// Package service contains all business logic.
// Services orchestrate repositories, external integrations, and
// cross-cutting concerns like audit logging.
//
// A service method answers the question:
// "What should happen when X occurs?" — not "How do I query the database?"
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/internal/model"
	"github.com/cappyHoding/ptdpn-eform-service/internal/repository"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/crypto"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/jwt"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	"go.uber.org/zap"
)

// ─── Input / Output DTOs ──────────────────────────────────────────────────────
// DTOs (Data Transfer Objects) are simple structs that carry data between
// layers. They're different from models — models represent DB rows,
// DTOs represent what goes IN and OUT of service methods.

// LoginInput holds the data needed to authenticate a staff member.
type LoginInput struct {
	Username  string
	Password  string
	IPAddress string
	UserAgent string
}

// LoginOutput holds the result of a successful login.
type LoginOutput struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    int64       `json:"expires_at"`
	User         UserSummary `json:"user"`
}

// UserSummary is a safe, minimal view of an internal user for API responses.
// It deliberately excludes the password hash.
type UserSummary struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// RefreshInput holds the data needed to refresh an access token.
type RefreshInput struct {
	RefreshToken string
	IPAddress    string
}

// ─── Service Interface ────────────────────────────────────────────────────────

// AuthService defines all authentication operations for internal staff.
type AuthService interface {
	Login(ctx context.Context, input LoginInput) (*LoginOutput, error)
	Logout(ctx context.Context, userID, username, ipAddress string) error
	RefreshToken(ctx context.Context, input RefreshInput) (*LoginOutput, error)
	GetUserByID(ctx context.Context, id string) (*UserSummary, error)
}

// ─── Service Errors ───────────────────────────────────────────────────────────
// Typed errors allow handlers to return appropriate HTTP status codes
// without the service layer knowing anything about HTTP.

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// ─── Implementation ───────────────────────────────────────────────────────────

// authService is the concrete implementation of AuthService.
type authService struct {
	userRepo  repository.UserRepository
	auditRepo repository.AuditRepository
	jwt       *jwt.Manager
	log       *logger.Logger
}

// NewAuthService creates a new AuthService with all required dependencies.
// This is called once at startup in main.go and the result is injected
// into the AuthHandler.
func NewAuthService(
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	jwtManager *jwt.Manager,
	log *logger.Logger,
) AuthService {
	return &authService{
		userRepo:  userRepo,
		auditRepo: auditRepo,
		jwt:       jwtManager,
		log:       log,
	}
}

// Login authenticates an internal staff member and returns a JWT token pair.
//
// BUSINESS RULES:
//  1. Username must exist and not be soft-deleted
//  2. Password must match the stored bcrypt hash
//  3. Account must be active (is_active = true)
//  4. Both success and failure are written to audit_logs
//
// SECURITY NOTE:
// We return the same error message for "user not found" and "wrong password"
// intentionally. Different messages would allow attackers to enumerate
// valid usernames (user enumeration attack).
func (s *authService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	// ── Look up the user ──────────────────────────────────────────────────────
	user, err := s.userRepo.FindByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// Log failed attempt even for non-existent users
			s.writeAuditLog(ctx, &model.AuditLog{
				ActorType:     "internal_user",
				ActorUsername: strPtr(input.Username),
				Action:        "LOGIN_FAILED",
				Description:   strPtr("Login failed: username not found"),
				IPAddress:     strPtr(input.IPAddress),
				UserAgent:     strPtr(input.UserAgent),
			})
			// Return generic error — do not reveal that the username doesn't exist
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("login lookup failed: %w", err)
	}

	// ── Verify password ───────────────────────────────────────────────────────
	if err := crypto.CheckPassword(input.Password, user.Password); err != nil {
		s.writeAuditLog(ctx, &model.AuditLog{
			ActorType:     "internal_user",
			ActorID:       strPtr(user.ID),
			ActorUsername: strPtr(user.Username),
			ActorRole:     strPtr(user.Role.Name),
			Action:        "LOGIN_FAILED",
			Description:   strPtr("Login failed: incorrect password"),
			IPAddress:     strPtr(input.IPAddress),
			UserAgent:     strPtr(input.UserAgent),
		})
		return nil, ErrInvalidCredentials
	}

	// ── Check account is active ───────────────────────────────────────────────
	if !user.IsActive {
		s.writeAuditLog(ctx, &model.AuditLog{
			ActorType:     "internal_user",
			ActorID:       strPtr(user.ID),
			ActorUsername: strPtr(user.Username),
			ActorRole:     strPtr(user.Role.Name),
			Action:        "LOGIN_FAILED",
			Description:   strPtr("Login failed: account is inactive"),
			IPAddress:     strPtr(input.IPAddress),
			UserAgent:     strPtr(input.UserAgent),
		})
		return nil, ErrAccountInactive
	}

	// ── Generate token pair ───────────────────────────────────────────────────
	tokenPair, err := s.jwt.GenerateTokenPair(user.ID, user.Username, user.Role.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// ── Write success audit log ───────────────────────────────────────────────
	s.writeAuditLog(ctx, &model.AuditLog{
		ActorType:     "internal_user",
		ActorID:       strPtr(user.ID),
		ActorUsername: strPtr(user.Username),
		ActorRole:     strPtr(user.Role.Name),
		Action:        "LOGIN_SUCCESS",
		Description:   strPtr("Staff member logged in successfully"),
		IPAddress:     strPtr(input.IPAddress),
		UserAgent:     strPtr(input.UserAgent),
	})

	s.log.Info("Staff login successful",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("role", user.Role.Name),
		zap.String("ip", input.IPAddress),
	)

	return &LoginOutput{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User: UserSummary{
			ID:       user.ID,
			Username: user.Username,
			FullName: user.FullName,
			Email:    user.Email,
			Role:     user.Role.Name,
		},
	}, nil
}

// Logout records the logout event in the audit log.
//
// NOTE: Because we use stateless JWT tokens, we can't truly "invalidate" a token
// server-side without a blocklist. For now, logout is a client-side operation
// (the frontend discards the token). The audit log records the intent.
//
// In a future phase, we can add a Redis token blocklist to enforce server-side
// invalidation before the token naturally expires.
func (s *authService) Logout(ctx context.Context, userID, username, ipAddress string) error {
	s.writeAuditLog(ctx, &model.AuditLog{
		ActorType:     "internal_user",
		ActorID:       strPtr(userID),
		ActorUsername: strPtr(username),
		Action:        "LOGOUT",
		Description:   strPtr("Staff member logged out"),
		IPAddress:     strPtr(ipAddress),
	})

	s.log.Info("Staff logout",
		zap.String("user_id", userID),
		zap.String("username", username),
	)

	return nil
}

// RefreshToken validates a refresh token and issues a new access token.
// The user's current role is re-fetched from the DB to catch role changes.
func (s *authService) RefreshToken(ctx context.Context, input RefreshInput) (*LoginOutput, error) {
	// Verify the refresh token and extract the user ID
	userID, err := s.jwt.VerifyRefreshToken(input.RefreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Re-fetch the user to get current role and check they're still active
	// This is important: if an admin deactivates a user, their refresh token
	// should no longer work even if it hasn't expired yet
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("refresh token user lookup failed: %w", err)
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	// Issue new token pair
	tokenPair, err := s.jwt.GenerateTokenPair(user.ID, user.Username, user.Role.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens on refresh: %w", err)
	}

	s.writeAuditLog(ctx, &model.AuditLog{
		ActorType:     "internal_user",
		ActorID:       strPtr(user.ID),
		ActorUsername: strPtr(user.Username),
		ActorRole:     strPtr(user.Role.Name),
		Action:        "TOKEN_REFRESHED",
		Description:   strPtr("Access token refreshed"),
		IPAddress:     strPtr(input.IPAddress),
	})

	return &LoginOutput{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User: UserSummary{
			ID:       user.ID,
			Username: user.Username,
			FullName: user.FullName,
			Email:    user.Email,
			Role:     user.Role.Name,
		},
	}, nil
}

// GetUserByID retrieves a safe summary of an internal user.
// Used by the frontend to load the current user's profile.
func (s *authService) GetUserByID(ctx context.Context, id string) (*UserSummary, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserSummary{
		ID:       user.ID,
		Username: user.Username,
		FullName: user.FullName,
		Email:    user.Email,
		Role:     user.Role.Name,
	}, nil
}

// ─── Private Helpers ──────────────────────────────────────────────────────────

// writeAuditLog writes an audit entry and logs (but does not propagate)
// any error. Audit log failures should never interrupt the main request flow.
func (s *authService) writeAuditLog(ctx context.Context, entry *model.AuditLog) {
	// Set timestamp explicitly
	entry.CreatedAt = time.Now()

	if err := s.auditRepo.Write(ctx, entry); err != nil {
		// Log the failure but don't return it — audit log writes are
		// best-effort and should not block the user's request
		s.log.Error("Failed to write audit log",
			zap.String("action", entry.Action),
			zap.Error(err),
		)
	}
}

// strPtr is a helper to get a pointer to a string literal.
// Required because Go doesn't allow taking the address of a literal directly.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
