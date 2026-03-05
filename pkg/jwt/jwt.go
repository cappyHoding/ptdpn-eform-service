// Package jwt handles creation and verification of JWT tokens
// for internal staff (operators, supervisors, admins).
//
// ALGORITHM: RS256 (RSA Signature with SHA-256)
//   - Private key: used to SIGN tokens (kept secret on server)
//   - Public key:  used to VERIFY tokens (can be shared)
//
// TOKEN PAIR STRATEGY:
//   - Access Token  (15 min TTL): Short-lived, sent with every API request
//   - Refresh Token (24 hr TTL):  Long-lived, only used to get a new access token
//
// This means: if an access token is stolen, it expires quickly.
// If you need to revoke a user immediately, you can add their
// user ID to a Redis blocklist checked during verification.
package jwt

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Claims defines the data embedded inside the JWT token.
// These are called "claims" in JWT terminology.
//
// Standard claims (from RegisteredClaims) include:
//   - Subject (sub): the user's ID
//   - ExpiresAt (exp): when the token expires
//   - IssuedAt  (iat): when the token was created
//
// We add custom claims for role-based access control.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // "admin" | "supervisor" | "operator"
	gojwt.RegisteredClaims
}

// TokenPair holds both tokens returned at login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // Unix timestamp for frontend countdown
}

// Manager handles JWT signing and verification using RSA keys.
type Manager struct {
	privateKey      *rsa.PrivateKey
	publicKey       *rsa.PublicKey
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewManager loads RSA keys from disk and returns a ready-to-use Manager.
//
// Call this once at startup and inject the manager into your auth service.
// If the key files don't exist, generate them with:
//
//	mkdir -p keys
//	openssl genrsa -out keys/private.pem 2048
//	openssl rsa -in keys/private.pem -pubout -out keys/public.pem
func NewManager(privateKeyPath, publicKeyPath string, accessTTL, refreshTTL time.Duration) (*Manager, error) {
	// Load private key (for signing)
	privateBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key from %s: %w", privateKeyPath, err)
	}
	privateKey, err := gojwt.ParseRSAPrivateKeyFromPEM(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Load public key (for verification)
	publicBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key from %s: %w", publicKeyPath, err)
	}
	publicKey, err := gojwt.ParseRSAPublicKeyFromPEM(publicBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &Manager{
		privateKey:      privateKey,
		publicKey:       publicKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}, nil
}

// GenerateTokenPair creates a new access + refresh token pair for an authenticated user.
// Call this when a user successfully logs in.
func (m *Manager) GenerateTokenPair(userID, username, role string) (*TokenPair, error) {
	now := time.Now()

	// ── Build access token ────────────────────────────────────────────────────
	accessClaims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(m.accessTokenTTL)),
			// Issuer identifies who created the token — useful if you later add
			// multiple services that all verify the same token
			Issuer: "bpr-perdana-eform",
		},
	}
	accessToken := gojwt.NewWithClaims(gojwt.SigningMethodRS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// ── Build refresh token ───────────────────────────────────────────────────
	// Refresh tokens have minimal claims — just enough to identify the user
	// and issue a new access token. No role info needed.
	refreshClaims := gojwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  gojwt.NewNumericDate(now),
		ExpiresAt: gojwt.NewNumericDate(now.Add(m.refreshTokenTTL)),
		Issuer:    "bpr-perdana-eform",
		// We use "refresh" audience to prevent a refresh token being used
		// as an access token
		Audience: gojwt.ClaimStrings{"refresh"},
	}
	refreshToken := gojwt.NewWithClaims(gojwt.SigningMethodRS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    now.Add(m.accessTokenTTL).Unix(),
	}, nil
}

// VerifyAccessToken validates an access token and returns its claims.
// Returns an error if the token is expired, malformed, or has an invalid signature.
func (m *Manager) VerifyAccessToken(tokenString string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(
		tokenString,
		&Claims{},
		// The key function tells the JWT library which key to use for verification
		func(token *gojwt.Token) (interface{}, error) {
			// Verify the signing algorithm is what we expect
			// IMPORTANT: Without this check, an attacker could forge tokens
			// by changing the algorithm to "none" or switching to HS256
			if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.publicKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// VerifyRefreshToken validates a refresh token and returns the subject (user ID).
func (m *Manager) VerifyRefreshToken(tokenString string) (string, error) {
	token, err := gojwt.ParseWithClaims(
		tokenString,
		&gojwt.RegisteredClaims{},
		func(token *gojwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.publicKey, nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*gojwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid refresh token claims")
	}

	return claims.Subject, nil
}
