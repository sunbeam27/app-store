package app_store

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Token struct {
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
	token := jwt.New(jwt.SigningMethodES256)
	token.Header["kid"] = keyId
	now := time.Now()
	t.iat = now
	t.exp = now.Add(time.Hour)
	token.Claims = &jwt.MapClaims{
		"iss": issuer,
		"iat": t.iat,
		"exp": t.exp,
		"aud": "appstoreconnect-v1",
		"bid": bundleId,
	}
	tokenStr, err := token.SignedString(t.privateKey)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

func (t *Token) SignedString() string {
	token := t.token
	if time.Now().After(t.exp) {
		token, _ = t.generate(t.kid, t.issuer, t.bundle)
	}
	return token
}

func (t *Token) Bearer() string {
	token := t.token
	if time.Now().After(t.exp) {
		token, _ = t.generate(t.kid, t.issuer, t.bundle)
	}
	return fmt.Sprintf("Bearer %s", token)
}
