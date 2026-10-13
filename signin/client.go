package signin

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sunbeam27/app-store/internal/applejwt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AppleAuthURL   = "https://appleid.apple.com/auth/authorize"
	AppleTokenURL  = "https://appleid.apple.com/auth/token"
	AppleKeysURL   = "https://appleid.apple.com/auth/keys"
	AppleRevokeURL = "https://appleid.apple.com/auth/revoke"
	AppleIssuer    = "https://appleid.apple.com"
)

// AppleKey represents a single key from Apple's JWKS endpoint.
type AppleKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// AppleKeysResponse represents the response from Apple's JWKS endpoint.
type AppleKeysResponse struct {
	Keys []AppleKey `json:"keys"`
}

// AppleIDTokenClaims represents the claims in an Apple ID token.
type AppleIDTokenClaims struct {
	jwt.RegisteredClaims
	Email          string `json:"email,omitempty"`
	EmailVerified  any    `json:"email_verified,omitempty"`
	IsPrivateEmail any    `json:"is_private_email,omitempty"`
	RealUserStatus int    `json:"real_user_status,omitempty"`
	TransferSub    string `json:"transfer_sub,omitempty"`
	NonceSupported bool   `json:"nonce_supported,omitempty"`
	AuthTime       int64  `json:"auth_time,omitempty"`
	Nonce          string `json:"nonce,omitempty"`
	CHash          string `json:"c_hash,omitempty"`
	AtHash         string `json:"at_hash,omitempty"`
}

// IsEmailVerified returns whether the email is verified.
func (c *AppleIDTokenClaims) IsEmailVerified() bool {
	switch v := c.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

// AppleTokenResponse represents the response from Apple's token endpoint.
type AppleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// AppleAuthClient handles Sign in with Apple operations.
type AppleAuthClient struct {
	clientID   string
	teamID     string
	keyID      string
	privateKey string
	httpClient *http.Client
}

// NewAppleAuthClient creates a new Sign in with Apple client.
// clientID is the Services ID or App Bundle ID.
// teamID is the Apple Developer Team ID.
// keyID is the Key ID from the Apple Developer portal.
// privateKey is the PEM-encoded ECDSA private key content (.p8 file).
func NewAppleAuthClient(clientID, teamID, keyID, privateKey string) *AppleAuthClient {
	return &AppleAuthClient{
		clientID:   clientID,
		teamID:     teamID,
		keyID:      keyID,
		privateKey: privateKey,
		httpClient: http.DefaultClient,
	}
}

// WithHTTPClient sets a custom HTTP client.
func (a *AppleAuthClient) WithHTTPClient(client *http.Client) {
	a.httpClient = client
}

// GenerateClientSecret generates a client secret JWT for Sign in with Apple.
func (a *AppleAuthClient) GenerateClientSecret(expiry time.Duration) (string, error) {
	key, err := applejwt.ParseKey(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": a.teamID,
		"iat": now.Unix(),
		"exp": now.Add(expiry).Unix(),
		"aud": AppleIssuer,
		"sub": a.clientID,
	})
	token.Header["kid"] = a.keyID

	return token.SignedString(key)
}

// ExchangeCode exchanges an authorization code for tokens.
func (a *AppleAuthClient) ExchangeCode(code, redirectURI string) (*AppleTokenResponse, error) {
	return a.ExchangeCodeContext(context.Background(), code, redirectURI)
}

func (a *AppleAuthClient) ExchangeCodeContext(ctx context.Context, code, redirectURI string) (*AppleTokenResponse, error) {
	secret, err := a.GenerateClientSecret(5 * time.Minute)
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {a.clientID},
		"client_secret": {secret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	resp, err := a.postForm(ctx, AppleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple token error (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp AppleTokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

// RefreshAccessToken refreshes an access token using a refresh token.
func (a *AppleAuthClient) RefreshAccessToken(refreshToken string) (*AppleTokenResponse, error) {
	return a.RefreshAccessTokenContext(context.Background(), refreshToken)
}

func (a *AppleAuthClient) RefreshAccessTokenContext(ctx context.Context, refreshToken string) (*AppleTokenResponse, error) {
	secret, err := a.GenerateClientSecret(5 * time.Minute)
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {a.clientID},
		"client_secret": {secret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := a.postForm(ctx, AppleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple refresh error (status %d): %s", resp.StatusCode, body)
	}

	var tokenResp AppleTokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

// RevokeToken revokes an Apple token (access or refresh token).
func (a *AppleAuthClient) RevokeToken(token, tokenTypeHint string) error {
	return a.RevokeTokenContext(context.Background(), token, tokenTypeHint)
}

func (a *AppleAuthClient) RevokeTokenContext(ctx context.Context, token, tokenTypeHint string) error {
	secret, err := a.GenerateClientSecret(5 * time.Minute)
	if err != nil {
		return err
	}

	data := url.Values{
		"client_id":       {a.clientID},
		"client_secret":   {secret},
		"token":           {token},
		"token_type_hint": {tokenTypeHint},
	}

	resp, err := a.postForm(ctx, AppleRevokeURL, data)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("apple revoke error (status %d): %s", resp.StatusCode, body)
	}
	return nil
}

// VerifyIDToken verifies an Apple ID token and returns the decoded claims.
func (a *AppleAuthClient) VerifyIDToken(idToken string) (*AppleIDTokenClaims, error) {
	return VerifyAppleIDToken(a.httpClient, a.clientID, idToken)
}

// VerifyAppleIDToken verifies an Apple ID token using Apple's public keys.
// clientID is the expected audience (your Services ID or App Bundle ID).
func VerifyAppleIDToken(httpClient *http.Client, clientID, idToken string) (*AppleIDTokenClaims, error) {
	return VerifyAppleIDTokenContext(context.Background(), httpClient, clientID, idToken)
}

func VerifyAppleIDTokenContext(ctx context.Context, httpClient *http.Client, clientID, idToken string) (*AppleIDTokenClaims, error) {
	if clientID == "" {
		return nil, errors.New("expected client ID is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Fetch Apple's public keys
	keys, err := fetchApplePublicKeysContext(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("fetch apple keys: %w", err)
	}

	// Parse the token header to get the kid
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err = json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "RS256" || header.Kid == "" {
		return nil, errors.New("invalid token algorithm or key ID")
	}

	// Find matching key
	var matchedKey *AppleKey
	for i := range keys.Keys {
		if keys.Keys[i].Kid == header.Kid {
			matchedKey = &keys.Keys[i]
			break
		}
	}
	if matchedKey == nil {
		return nil, errors.New("no matching key found")
	}
	if matchedKey.Kty != "RSA" || matchedKey.Use != "sig" || matchedKey.Alg != "RS256" {
		return nil, errors.New("invalid Apple signing key")
	}

	// Build RSA public key
	pubKey, err := buildRSAPublicKey(matchedKey)
	if err != nil {
		return nil, fmt.Errorf("build public key: %w", err)
	}

	// Parse and verify the token
	claims := &AppleIDTokenClaims{}
	_, err = jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	}, jwt.WithIssuer(AppleIssuer), jwt.WithAudience(clientID), jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired())

	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}

	if claims.Subject == "" {
		return nil, errors.New("missing token subject")
	}
	return claims, nil
}

func fetchApplePublicKeys(client *http.Client) (*AppleKeysResponse, error) {
	return fetchApplePublicKeysContext(context.Background(), client)
}
func fetchApplePublicKeysContext(ctx context.Context, client *http.Client) (*AppleKeysResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AppleKeysURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple keys error (status %d): %s", resp.StatusCode, body)
	}
	var keys AppleKeysResponse
	if err = json.Unmarshal(body, &keys); err != nil {
		return nil, err
	}
	return &keys, nil
}

func buildRSAPublicKey(key *AppleKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// VerifyIDTokenWithNonce also binds the token to the authorization request.
func (a *AppleAuthClient) VerifyIDTokenWithNonce(idToken, expectedNonce string) (*AppleIDTokenClaims, error) {
	if expectedNonce == "" {
		return nil, errors.New("expected nonce is required")
	}
	claims, err := a.VerifyIDToken(idToken)
	if err != nil {
		return nil, err
	}
	if claims.Nonce != expectedNonce {
		return nil, errors.New("nonce mismatch")
	}
	return claims, nil
}

// VerifyIDTokenContext permits cancellation during public-key retrieval.
func (a *AppleAuthClient) VerifyIDTokenContext(ctx context.Context, token string) (*AppleIDTokenClaims, error) {
	return VerifyAppleIDTokenContext(ctx, a.httpClient, a.clientID, token)
}

func (a *AppleAuthClient) VerifyIDTokenWithNonceContext(ctx context.Context, token, nonce string) (*AppleIDTokenClaims, error) {
	if nonce == "" {
		return nil, errors.New("expected nonce is required")
	}
	claims, err := a.VerifyIDTokenContext(ctx, token)
	if err != nil {
		return nil, err
	}
	if claims.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}
	return claims, nil
}

func (a *AppleAuthClient) postForm(ctx context.Context, endpoint string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.httpClient.Do(req)
}
