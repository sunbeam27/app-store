package signin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sunbeam27/app-store/internal/applejwt"
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
		key, _ := applejwt.ParseKey(p8Key)
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
