package app_store

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewToken(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("test-kid", "test-issuer", "com.test.bundle", key)

	if token.kid != "test-kid" {
		t.Errorf("expected kid 'test-kid', got '%s'", token.kid)
	}
	if token.issuer != "test-issuer" {
		t.Errorf("expected issuer 'test-issuer', got '%s'", token.issuer)
	}
	if token.bundle != "com.test.bundle" {
		t.Errorf("expected bundle 'com.test.bundle', got '%s'", token.bundle)
	}
}

func TestTokenGenerate(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tk := NewToken("test-kid", "test-issuer", "com.test.bundle", key)

	tokenStr, err := tk.generate("test-kid", "test-issuer", "com.test.bundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token should not be empty")
	}

	// Verify the token can be parsed
	parsed, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to get claims")
	}

	if claims["iss"] != "test-issuer" {
		t.Errorf("expected iss 'test-issuer', got '%v'", claims["iss"])
	}
	if claims["aud"] != "appstoreconnect-v1" {
		t.Errorf("expected aud 'appstoreconnect-v1', got '%v'", claims["aud"])
	}
	if claims["bid"] != "com.test.bundle" {
		t.Errorf("expected bid 'com.test.bundle', got '%v'", claims["bid"])
	}
}

func TestTokenSignedString(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tk := NewToken("test-kid", "test-issuer", "com.test.bundle", key)

	// Pre-generate and store the token
	tk.token, _ = tk.generate(tk.kid, tk.issuer, tk.bundle)

	tokenStr := tk.SignedString()
	if tokenStr == "" {
		t.Fatal("token should not be empty")
	}

	// Should return the same stored token within expiry
	tokenStr2 := tk.SignedString()
	if tokenStr != tokenStr2 {
		t.Error("expected same token within expiry period")
	}
}

func TestTokenBearer(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tk := NewToken("test-kid", "test-issuer", "com.test.bundle", key)

	// Force token generation
	tk.token, _ = tk.generate(tk.kid, tk.issuer, tk.bundle)

	bearer := tk.Bearer()
	if !strings.HasPrefix(bearer, "Bearer ") {
		t.Errorf("expected bearer prefix, got '%s'", bearer)
	}
}

func TestTokenExpiry(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tk := NewToken("test-kid", "test-issuer", "com.test.bundle", key)

	// Generate with known expiry
	tk.token, _ = tk.generate(tk.kid, tk.issuer, tk.bundle)

	// Token should expire in about 1 hour
	if tk.exp.Before(time.Now()) {
		t.Error("token should not be expired immediately after generation")
	}
	if tk.exp.After(time.Now().Add(2 * time.Hour)) {
		t.Error("token expiry should be within 2 hours")
	}
}
