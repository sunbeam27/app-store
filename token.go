package app_store

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"sync"
	"time"
)

type Token struct {
	mu                  sync.Mutex
	token               string
	kid, issuer, bundle string
	iat                 time.Time
	exp                 time.Time
	privateKey          *ecdsa.PrivateKey
}

func NewToken(keyId, issuer, bundle string, privateKey *ecdsa.PrivateKey) *Token {
	t := &Token{
		kid:        keyId,
		issuer:     issuer,
		bundle:     bundle,
		privateKey: privateKey,
	}
	return t
}

func (t *Token) generate(keyId, issuer, bundleId string) (string, error) {
	if t.privateKey == nil {
		return "", fmt.Errorf("private key is required")
	}
	token := jwt.New(jwt.SigningMethodES256)
	token.Header["kid"] = keyId
	now := time.Now()
	t.iat = now
	t.exp = now.Add(time.Hour)
	token.Claims = jwt.MapClaims{
		"iss": issuer,
		"iat": t.iat.Unix(),
		"exp": t.exp.Unix(),
		"aud": "appstoreconnect-v1",
		"bid": bundleId,
	}
	tokenStr, err := token.SignedString(t.privateKey)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

// SignedString returns the cached token, or an empty string if signing fails.
func (t *Token) SignedString() string {
	token, _ := t.signedString()
	return token
}

func (t *Token) signedString() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token == "" || !time.Now().Before(t.exp) {
		token, err := t.generate(t.kid, t.issuer, t.bundle)
		if err != nil {
			return "", err
		}
		t.token = token
	}
	return t.token, nil
}

func (t *Token) Bearer() string {
	token, err := t.signedString()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("Bearer %s", token)
}

func (t *Token) bearer() (string, error) {
	token, err := t.signedString()
	if err != nil {
		return "", err
	}
	return "Bearer " + token, nil
}
