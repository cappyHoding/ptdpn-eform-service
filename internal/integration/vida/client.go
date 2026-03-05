package vida

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client adalah base HTTP client untuk semua VIDA service.
//
// PENTING: Token endpoint VIDA menggunakan application/x-www-form-urlencoded
// bukan JSON. Semua endpoint lainnya menggunakan JSON.
type Client struct {
	baseURL      string
	ssoURL       string
	clientID     string
	clientSecret string
	scope        string
	httpClient   *http.Client

	mu         sync.Mutex
	tokenCache *cachedToken
}

// NewClient membuat Client baru.
func NewClient(baseURL, ssoURL, clientID, clientSecret, scope string, timeout time.Duration) *Client {
	return &Client{
		baseURL:      baseURL,
		ssoURL:       ssoURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        scope,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tokenCache != nil && time.Now().Before(c.tokenCache.expiresAt.Add(-30*time.Second)) {
		return c.tokenCache.token, nil
	}
	return c.fetchNewToken(ctx)
}

// fetchNewToken melakukan POST ke token endpoint dengan urlencoded body.
func (c *Client) fetchNewToken(ctx context.Context) (string, error) {
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", c.clientID)
	formData.Set("client_secret", c.clientSecret)
	formData.Set("scope", c.scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.ssoURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	c.tokenCache = &cachedToken{
		token:     tokenResp.AccessToken,
		expiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}
	return tokenResp.AccessToken, nil
}

func (c *Client) GetAccessToken(ctx context.Context) (string, error) {
	return c.getAccessToken(ctx)
}

func (c *Client) GetTokenInfo(ctx context.Context) (token string, expiresAt int64, expiresIn int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Gunakan cache jika masih valid
	if c.tokenCache != nil && time.Now().Before(c.tokenCache.expiresAt.Add(-30*time.Second)) {
		remaining := int(time.Until(c.tokenCache.expiresAt).Seconds())
		return c.tokenCache.token, c.tokenCache.expiresAt.Unix(), remaining, nil
	}

	// Fetch baru
	tok, err := c.fetchNewToken(ctx)
	if err != nil {
		return "", 0, 0, err
	}

	remaining := int(time.Until(c.tokenCache.expiresAt).Seconds())
	return tok, c.tokenCache.expiresAt.Unix(), remaining, nil
}

// doJSON melakukan HTTP request dengan JSON body.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	return c.doJSONWithHeaders(ctx, method, path, nil, body, result)
}

// doJSONWithHeaders melakukan HTTP request dengan custom headers tambahan.
func (c *Client) doJSONWithHeaders(ctx context.Context, method, path string, extraHeaders map[string]string, body interface{}, result interface{}) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, result)
}

func (c *Client) parseResponse(resp *http.Response, result interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return fmt.Errorf("VIDA API error (HTTP %d): %s (code: %s)",
				resp.StatusCode, errResp.Message, errResp.Code)
		}
		return fmt.Errorf("VIDA API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to parse VIDA response: %w\nbody: %s", err, string(body))
		}
	}
	return nil
}

// PollUntilDone melakukan polling ke status endpoint sampai selesai.
// Dipakai oleh OCR dan Fraud API yang bersifat async.
func (c *Client) PollUntilDone(
	ctx context.Context,
	statusPath string,
	isDone func(body []byte) bool,
	result interface{},
	interval time.Duration,
	maxAttempts int,
) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return err
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("polling cancelled: %w", ctx.Err())
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+statusPath, nil)
		if err != nil {
			return fmt.Errorf("failed to create poll request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll request failed: %w", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("failed to read poll response: %w", readErr)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("poll returned HTTP %d: %s", resp.StatusCode, string(body))
		}

		if isDone(body) {
			if result != nil {
				if err := json.Unmarshal(body, result); err != nil {
					return fmt.Errorf("failed to parse final poll response: %w", err)
				}
			}
			return nil
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("polling cancelled while waiting: %w", ctx.Err())
			case <-time.After(interval):
			}
		}
	}

	return fmt.Errorf("polling timed out after %d attempts", maxAttempts)
}

// TestTokenFetch adalah exported helper untuk test script.
// Melakukan token fetch dan mengembalikan error jika gagal.
func (c *Client) TestTokenFetch(ctx context.Context) error {
	_, err := c.getAccessToken(ctx)
	return err
}

// DoRawGet melakukan GET request dan mengembalikan raw body + status code.
// Dipakai oleh FraudService untuk polling dengan logging per-attempt.
func (c *Client) DoRawGet(ctx context.Context, path string) ([]byte, int, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.StatusCode, nil
}

func (c *Client) GetTokenWithCredentials(ctx context.Context, clientID, clientSecret string) (token string, expiresAt int64, expiresIn int, err error) {
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", clientID)
	formData.Set("client_secret", clientSecret)
	formData.Set("scope", c.scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.ssoURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse token response: %w", err)
	}

	expAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	return tokenResp.AccessToken, expAt, tokenResp.ExpiresIn, nil
}
