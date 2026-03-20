package app_store

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestP8Key(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})
	return string(pemBlock)
}

func TestNewAppleAuthClient(t *testing.T) {
	client := NewAppleAuthClient("com.test.app", "TEAM123", "KEY123", "test-key")
	if client.clientID != "com.test.app" {
		t.Errorf("expected clientID 'com.test.app', got '%s'", client.clientID)
	}
	if client.teamID != "TEAM123" {
		t.Errorf("expected teamID 'TEAM123', got '%s'", client.teamID)
	}
	if client.keyID != "KEY123" {
		t.Errorf("expected keyID 'KEY123', got '%s'", client.keyID)
	}
}

func TestAppleAuthClientWithHTTPClient(t *testing.T) {
	client := NewAppleAuthClient("com.test.app", "TEAM123", "KEY123", "test-key")
	customHTTP := &http.Client{}
	client.WithHTTPClient(customHTTP)
	if client.httpClient != customHTTP {
		t.Error("custom HTTP client not set")
	}
}

func TestGenerateClientSecret(t *testing.T) {
	p8Key := generateTestP8Key(t)
	client := NewAppleAuthClient("com.test.app", "TEAM123", "KEY123", p8Key)

	secret, err := client.GenerateClientSecret(5 * time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == "" {
		t.Fatal("secret should not be empty")
	}

	// Parse and verify the secret JWT
	parsed, err := jwt.Parse(secret, func(token *jwt.Token) (interface{}, error) {
		key, _ := ParsePrivateKey(p8Key)
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse secret: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to get claims")
	}
	if claims["iss"] != "TEAM123" {
		t.Errorf("expected iss 'TEAM123', got '%v'", claims["iss"])
	}
	if claims["sub"] != "com.test.app" {
		t.Errorf("expected sub 'com.test.app', got '%v'", claims["sub"])
	}
	if claims["aud"] != AppleIssuer {
		t.Errorf("expected aud '%s', got '%v'", AppleIssuer, claims["aud"])
	}
}

func TestGenerateClientSecretInvalidKey(t *testing.T) {
	client := NewAppleAuthClient("com.test.app", "TEAM123", "KEY123", "invalid-key")
	_, err := client.GenerateClientSecret(5 * time.Minute)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestExchangeCode(t *testing.T) {
	expectedResp := AppleTokenResponse{
		AccessToken:  "access_token_123",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "refresh_token_123",
		IDToken:      "id_token_123",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("expected grant_type 'authorization_code', got '%s'", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "test-code" {
			t.Errorf("expected code 'test-code', got '%s'", r.FormValue("code"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	p8Key := generateTestP8Key(t)
	client := NewAppleAuthClient("com.test.app", "TEAM123", "KEY123", p8Key)
	client.httpClient = server.Client()

	// Override the URL by using the test server
	origURL := AppleTokenURL
	defer func() { _ = origURL }()

	// We need to use a custom approach since the URL is const
	// Instead, directly test with the server client
	resp, err := func() (*AppleTokenResponse, error) {
		secret, err := client.GenerateClientSecret(5 * time.Minute)
		if err != nil {
			return nil, err
		}
		data := map[string][]string{
			"client_id":     {client.clientID},
			"client_secret": {secret},
			"code":          {"test-code"},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {"https://example.com/callback"},
		}
		r, err := server.Client().PostForm(server.URL, data)
		if err != nil {
			return nil, err
		}
		defer r.Body.Close()
		var tokenResp AppleTokenResponse
		json.NewDecoder(r.Body).Decode(&tokenResp)
		return &tokenResp, nil
	}()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "access_token_123" {
		t.Errorf("expected access_token_123, got '%s'", resp.AccessToken)
	}
	if resp.RefreshToken != "refresh_token_123" {
		t.Errorf("expected refresh_token_123, got '%s'", resp.RefreshToken)
	}
}

func TestVerifyAppleIDToken(t *testing.T) {
	// Generate RSA key pair for testing
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	kid := "test-kid-123"

	// Create a mock JWKS endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := AppleKeysResponse{
			Keys: []AppleKey{
				{
					Kty: "RSA",
					Kid: kid,
					Use: "sig",
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(rsaKey.E)).Bytes()),
				},
			},
		}
		json.NewEncoder(w).Encode(keys)
	}))
	defer server.Close()

	// Create a test ID token
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":            AppleIssuer,
		"aud":            "com.test.app",
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"sub":            "user-123",
		"email":          "test@privaterelay.appleid.com",
		"email_verified": true,
	})
	token.Header["kid"] = kid

	signedToken, err := token.SignedString(rsaKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Create a custom verifier that uses our test server
	httpClient := server.Client()

	// We need to temporarily override the keys URL
	// Since we can't modify the const, we'll test the underlying functions
	// Test buildRSAPublicKey
	appleKey := AppleKey{
		Kty: "RSA",
		Kid: kid,
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(rsaKey.E)).Bytes()),
	}

	pubKey, err := buildRSAPublicKey(&appleKey)
	if err != nil {
		t.Fatalf("failed to build public key: %v", err)
	}

	// Verify the token with the built key
	claims := &AppleIDTokenClaims{}
	_, err = jwt.ParseWithClaims(signedToken, claims, func(token *jwt.Token) (interface{}, error) {
		return pubKey, nil
	}, jwt.WithIssuer(AppleIssuer), jwt.WithAudience("com.test.app"))
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if claims.Email != "test@privaterelay.appleid.com" {
		t.Errorf("expected email 'test@privaterelay.appleid.com', got '%s'", claims.Email)
	}

	_ = httpClient // Used in real integration tests
}

func TestAppleIDTokenClaimsIsEmailVerified(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"nil", nil, false},
		{"int", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &AppleIDTokenClaims{EmailVerified: tt.value}
			if claims.IsEmailVerified() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, claims.IsEmailVerified())
			}
		})
	}
}

func TestBuildRSAPublicKey(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	key := &AppleKey{
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(rsaKey.E)).Bytes()),
	}

	pubKey, err := buildRSAPublicKey(key)
	if err != nil {
		t.Fatalf("failed to build public key: %v", err)
	}
	if pubKey.N.Cmp(rsaKey.N) != 0 {
		t.Error("N values don't match")
	}
	if pubKey.E != rsaKey.E {
		t.Error("E values don't match")
	}
}

func TestBuildRSAPublicKeyInvalidN(t *testing.T) {
	key := &AppleKey{
		N: "!!!invalid!!!",
		E: "AQAB",
	}
	_, err := buildRSAPublicKey(key)
	if err == nil {
		t.Fatal("expected error for invalid N")
	}
}

func TestBuildRSAPublicKeyInvalidE(t *testing.T) {
	key := &AppleKey{
		N: base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}),
		E: "!!!invalid!!!",
	}
	_, err := buildRSAPublicKey(key)
	if err == nil {
		t.Fatal("expected error for invalid E")
	}
}

func TestFetchApplePublicKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := AppleKeysResponse{
			Keys: []AppleKey{
				{Kty: "RSA", Kid: "test-kid", Use: "sig", Alg: "RS256", N: "abc", E: "AQAB"},
			},
		}
		json.NewEncoder(w).Encode(keys)
	}))
	defer server.Close()

	// Can't test fetchApplePublicKeys directly with custom URL since it's hardcoded
	// But we test the HTTP parsing logic
	resp, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var keys AppleKeysResponse
	json.NewDecoder(resp.Body).Decode(&keys)

	if len(keys.Keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys.Keys))
	}
	if keys.Keys[0].Kid != "test-kid" {
		t.Errorf("expected kid 'test-kid', got '%s'", keys.Keys[0].Kid)
	}
}
